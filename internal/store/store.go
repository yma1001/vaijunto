package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yma1001/vaijunto/internal/domain"
	"github.com/yma1001/vaijunto/internal/protocol"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrNoSeats          = errors.New("no seats")
	ErrForbidden        = errors.New("forbidden")
	ErrValidation       = errors.New("validation")
	ErrAlreadyExists    = errors.New("already exists")
	ErrAlreadyCancelled = errors.New("already cancelled")
	ErrConflict         = errors.New("conflict")
)

// Store é o estado canônico do servidor.
//
// Concorrência: um único sync.RWMutex protege usuários, caronas, reservas e
// o índice de idempotência. Leituras (busca, listagens) usam RLock — vários
// clientes leem juntos. Escritas (publicar, confirmar, cancelar) usam Lock
// exclusivo. Com uma única trava não há deadlock por ordem de aquisição:
// confirmar um itinerário de várias caronas entra numa seção crítica só e
// revalida TODOS os trechos antes de alterar qualquer um (atomicidade).
//
// A persistência JSON é gravada ainda sob o Lock de escrita, depois da
// mutação, para que dois escritores não interleaveiem o arquivo. O arquivo
// NÃO é um lock: o mutex é quem serializa.
type Store struct {
	mu sync.RWMutex

	users        map[string]domain.User // username -> user
	usersByID    map[string]domain.User
	rides        map[string]domain.Ride
	reservations map[string]domain.Reservation

	// confirmIndex[passengerID+"|"+requestId] = reservationId
	// Impede double-booking por retransmissão do mesmo CONFIRM.
	confirmIndex map[string]string

	path string
	now  func() time.Time
}

type persistedState struct {
	Users        []domain.User        `json:"users"`
	Rides        []domain.Ride        `json:"rides"`
	Reservations []domain.Reservation `json:"reservations"`
	ConfirmIndex map[string]string    `json:"confirmIndex"`
}

func New(path string) (*Store, error) {
	abs, err := resolveDataPath(path)
	if err != nil {
		return nil, err
	}
	s := &Store{
		users:        map[string]domain.User{},
		usersByID:    map[string]domain.User{},
		rides:        map[string]domain.Ride{},
		reservations: map[string]domain.Reservation{},
		confirmIndex: map[string]string{},
		path:         abs,
		now:          time.Now,
	}
	if err := s.loadOrSeed(); err != nil {
		return nil, err
	}
	return s, nil
}

// resolveDataPath torna DATA_PATH absoluto em relação ao cwd do processo.
// Caminho relativo ainda depende de onde o servidor foi iniciado: use
// caminho absoluto (ou o volume Docker /data) para não criar um segundo state.json.
func resolveDataPath(path string) (string, error) {
	if path == "" {
		path = "data/state.json"
	}
	return filepath.Abs(path)
}

func (s *Store) Path() string { return s.path }

func (s *Store) loadOrSeed() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.seedUsersLocked()
			return s.persistLocked()
		}
		return err
	}
	var st persistedState
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("invalid state file: %w", err)
	}
	for _, u := range st.Users {
		s.users[u.Username] = u
	}
	for _, r := range st.Rides {
		s.rides[r.RideID] = r
	}
	// Não pular reservas no unmarshal: cancelled, sem requestId, etc. entram no mapa.
	for _, r := range st.Reservations {
		s.reservations[r.ReservationID] = r
	}
	s.rebuildIndexesLocked()
	if len(s.users) == 0 {
		s.seedUsersLocked()
		return s.persistLocked()
	}
	return nil
}

func (s *Store) seedUsersLocked() {
	seeds := []domain.User{
		{UserID: "user-driver-1", Username: "motorista1", Password: "senha123", Role: protocol.RoleDriver},
		{UserID: "user-driver-2", Username: "motorista2", Password: "senha123", Role: protocol.RoleDriver},
		{UserID: "user-pass-1", Username: "passageiro1", Password: "senha123", Role: protocol.RolePassenger},
		{UserID: "user-pass-2", Username: "passageiro2", Password: "senha123", Role: protocol.RolePassenger},
		{UserID: "user-pass-3", Username: "passageiro3", Password: "senha123", Role: protocol.RolePassenger},
	}
	for _, u := range seeds {
		s.users[u.Username] = u
	}
	s.rebuildIndexesLocked()
}

// rebuildIndexesLocked reconstrói usersByID e o índice de idempotência a partir
// dos mapas canônicos. confirmIndex persistido pode estar vazio/desatualizado;
// a fonte de verdade no load são as reservas.
func (s *Store) rebuildIndexesLocked() {
	s.usersByID = make(map[string]domain.User, len(s.users))
	for _, u := range s.users {
		s.usersByID[u.UserID] = u
	}
	s.confirmIndex = make(map[string]string, len(s.reservations))
	for _, r := range s.reservations {
		if r.RequestID == "" || r.ReservationID == "" {
			continue
		}
		s.confirmIndex[confirmKey(r.PassengerID, r.RequestID)] = r.ReservationID
	}
}

func (s *Store) persistLocked() error {
	st := persistedState{
		Users:        make([]domain.User, 0, len(s.users)),
		Rides:        make([]domain.Ride, 0, len(s.rides)),
		Reservations: make([]domain.Reservation, 0, len(s.reservations)),
		ConfirmIndex: map[string]string{},
	}
	for _, u := range s.users {
		st.Users = append(st.Users, u)
	}
	for _, r := range s.rides {
		st.Rides = append(st.Rides, r)
	}
	for _, r := range s.reservations {
		st.Reservations = append(st.Reservations, r)
	}
	for k, v := range s.confirmIndex {
		st.ConfirmIndex[k] = v
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	// Rename no mesmo filesystem é atômico: leitores veem o arquivo antigo
	// ou o novo, nunca um JSON pela metade.
	return os.Rename(tmp, s.path)
}

func newID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

func (s *Store) Authenticate(username, password string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok || u.Password != password {
		return domain.User{}, ErrForbidden
	}
	return domain.CopyUser(u), nil
}

func (s *Store) UserByID(id string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.usersByID[id]
	return domain.CopyUser(u), ok
}

func (s *Store) Register(username, password, role string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if username == "" || password == "" {
		return domain.User{}, fmt.Errorf("%w: username and password required", ErrValidation)
	}
	if role != protocol.RoleDriver && role != protocol.RolePassenger {
		return domain.User{}, fmt.Errorf("%w: role must be DRIVER or PASSENGER", ErrValidation)
	}
	if _, exists := s.users[username]; exists {
		return domain.User{}, ErrAlreadyExists
	}
	u := domain.User{
		UserID:   newID("user-"),
		Username: username,
		Password: password,
		Role:     role,
	}
	s.users[username] = u
	s.usersByID[u.UserID] = u
	if err := s.persistLocked(); err != nil {
		delete(s.users, username)
		delete(s.usersByID, u.UserID)
		return domain.User{}, err
	}
	return u, nil
}

func (s *Store) SnapshotRides() []domain.Ride {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Ride, 0, len(s.rides))
	for _, r := range s.rides {
		out = append(out, domain.CopyRide(r))
	}
	return out
}

func (s *Store) ListDriverRides(driverID string) []domain.Ride {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Ride{}
	for _, r := range s.rides {
		if r.DriverID == driverID {
			out = append(out, domain.CopyRide(r))
		}
	}
	return out
}

func (s *Store) PublishRide(driverID string, cities []string, date, timeOfDay string, capacity int, prices []int64) (domain.Ride, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(cities) < 2 {
		return domain.Ride{}, fmt.Errorf("%w: route needs at least two cities", ErrValidation)
	}
	for i := range cities {
		cities[i] = domain.NormalizeCity(cities[i])
		if cities[i] == "" {
			return domain.Ride{}, fmt.Errorf("%w: empty city", ErrValidation)
		}
	}
	for i := 0; i < len(cities); i++ {
		for j := i + 1; j < len(cities); j++ {
			if domain.CitiesEqual(cities[i], cities[j]) {
				return domain.Ride{}, fmt.Errorf("%w: duplicate city in route", ErrValidation)
			}
		}
	}
	if capacity < 1 {
		return domain.Ride{}, fmt.Errorf("%w: capacity must be >= 1", ErrValidation)
	}
	if len(prices) != len(cities)-1 {
		return domain.Ride{}, fmt.Errorf("%w: segmentPrices length must be len(cities)-1", ErrValidation)
	}
	for _, p := range prices {
		if p < 0 {
			return domain.Ride{}, fmt.Errorf("%w: price cannot be negative", ErrValidation)
		}
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return domain.Ride{}, fmt.Errorf("%w: departureDate must be YYYY-MM-DD", ErrValidation)
	}
	if _, err := time.Parse("15:04", timeOfDay); err != nil {
		return domain.Ride{}, fmt.Errorf("%w: departureTime must be HH:MM", ErrValidation)
	}

	id := newID("ride-")
	segs := make([]domain.Segment, len(cities)-1)
	for i := 0; i < len(cities)-1; i++ {
		segs[i] = domain.Segment{
			RideID:         id,
			SegmentIndex:   i,
			Origin:         cities[i],
			Destination:    cities[i+1],
			Price:          prices[i],
			AvailableSeats: capacity,
		}
	}
	ride := domain.Ride{
		RideID:        id,
		DriverID:      driverID,
		Cities:        domain.CopyStrings(cities),
		DepartureDate: date,
		DepartureTime: timeOfDay,
		Capacity:      capacity,
		Status:        domain.RideActive,
		Segments:      segs,
		CreatedAt:     s.now(),
	}
	s.rides[id] = ride
	if err := s.persistLocked(); err != nil {
		delete(s.rides, id)
		return domain.Ride{}, err
	}
	return domain.CopyRide(ride), nil
}

func (s *Store) ListReservations(passengerID string) []domain.Reservation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []domain.Reservation{}
	for _, r := range s.reservations {
		if r.PassengerID == passengerID {
			out = append(out, domain.CopyReservation(r))
		}
	}
	return out
}

func (s *Store) CheckInvariants() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.invariantsLocked()
}

func (s *Store) invariantsLocked() []string {
	var viol []string
	for _, ride := range s.rides {
		for _, seg := range ride.Segments {
			if seg.AvailableSeats < 0 {
				viol = append(viol, fmt.Sprintf("INV-1 %s[%d] available=%d", ride.RideID, seg.SegmentIndex, seg.AvailableSeats))
			}
			if seg.AvailableSeats > ride.Capacity {
				viol = append(viol, fmt.Sprintf("INV-2 %s[%d] available=%d capacity=%d", ride.RideID, seg.SegmentIndex, seg.AvailableSeats, ride.Capacity))
			}
		}
	}
	return viol
}
