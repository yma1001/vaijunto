package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/domain"
	"github.com/yma1001/vaijunto/internal/protocol"
	"github.com/yma1001/vaijunto/internal/service"
	"github.com/yma1001/vaijunto/internal/store"
)

// Session vive APENAS nesta conexão TCP. Reinício do processo descarta
// sessões; contas/caronas/reservas continuam no JSON. Sem token nas
// requisições: o papel está ligado ao socket autenticado.
type Session struct {
	Authenticated bool
	UserID        string
	Username      string
	Role          string
}

type Server struct {
	cfg  config.Config
	svc  *service.Service
	st   *store.Store
	log  *log.Logger

	ln net.Listener

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func New(cfg config.Config, st *store.Store, logger *log.Logger) *Server {
	if logger == nil {
		logger = logStd()
	}
	return &Server{
		cfg:   cfg,
		svc:   service.New(st, cfg.MaxTransfers, cfg.MaxResults),
		st:    st,
		log:   logger,
		conns: map[net.Conn]struct{}{},
	}
}

func logStd() *log.Logger {
	return log.New(os.Stdout, "[vaijunto] ", log.LstdFlags|log.Lmicroseconds)
}

func (s *Server) Store() *store.Store { return s.st }

func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr())
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	s.log.Printf("listening on %s (data=%s)", ln.Addr().String(), s.st.Path())
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Printf("accept error: %v", err)
			continue
		}
		s.track(conn, true)
		// Uma goroutine por conexão: atende vários clientes ao mesmo tempo.
		// Isolamento: panic/erro desta goroutine não derruba o Accept loop.
		go s.handleConnection(conn)
	}
}

func (s *Server) Close() error {
	s.mu.Lock()
	ln := s.ln
	s.ln = nil
	s.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		_ = c.Close()
	}
	return nil
}

func (s *Server) track(c net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		s.conns[c] = struct{}{}
	} else {
		delete(s.conns, c)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.track(conn, false)
	defer conn.Close()
	defer func() {
		if rec := recover(); rec != nil {
			s.log.Printf("connection panic isolated: %v from %s", rec, conn.RemoteAddr())
		}
	}()

	sess := &Session{}
	s.log.Printf("connected %s", conn.RemoteAddr())
	for {
		_ = conn.SetReadDeadline(time.Now().Add(s.cfg.IdleTimeout))
		raw, err := protocol.ReadFrame(conn, s.cfg.MaxPayloadBytes)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, protocol.ErrIncompleteFrame) {
				s.log.Printf("read %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
		resp := s.dispatch(sess, raw)
		payload, err := protocol.MarshalResponse(resp)
		if err != nil {
			s.log.Printf("marshal: %v", err)
			return
		}
		_ = conn.SetWriteDeadline(time.Now().Add(s.cfg.WriteTimeout))
		if err := protocol.WriteFrame(conn, payload); err != nil {
			s.log.Printf("write %s: %v", conn.RemoteAddr(), err)
			return
		}
		if sess.Role == "CLOSING" {
			return
		}
	}
}

func (s *Server) dispatch(sess *Session, raw []byte) protocol.Response {
	req, err := protocol.DecodeRequest(raw)
	if err != nil {
		return protocol.Error(protocol.PeekRequestID(raw), protocol.CodeInvalidJSON, "malformed JSON")
	}
	if err := req.Validate(); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, err.Error())
	}

	switch req.Operation {
	case protocol.OpPing:
		return protocol.OK(req.RequestID, protocol.PingResult{Message: "PONG"})
	case protocol.OpRegister:
		return s.opRegister(req)
	case protocol.OpLogin:
		return s.opLogin(sess, req)
	case protocol.OpLogout:
		return s.opLogout(sess, req)
	case protocol.OpPublishRide:
		if errResp, ok := s.require(sess, req, protocol.RoleDriver); !ok {
			return errResp
		}
		return s.opPublishRide(sess, req)
	case protocol.OpListDriverRides:
		if errResp, ok := s.require(sess, req, protocol.RoleDriver); !ok {
			return errResp
		}
		return s.opListDriverRides(sess, req)
	case protocol.OpCancelRide:
		if errResp, ok := s.require(sess, req, protocol.RoleDriver); !ok {
			return errResp
		}
		return s.opCancelRide(sess, req)
	case protocol.OpListRidePassengers:
		if errResp, ok := s.require(sess, req, protocol.RoleDriver); !ok {
			return errResp
		}
		return s.opListRidePassengers(sess, req)
	case protocol.OpSearchItineraries:
		if errResp, ok := s.require(sess, req, protocol.RolePassenger); !ok {
			return errResp
		}
		return s.opSearch(sess, req)
	case protocol.OpConfirmReservation:
		if errResp, ok := s.require(sess, req, protocol.RolePassenger); !ok {
			return errResp
		}
		return s.opConfirm(sess, req)
	case protocol.OpListReservations:
		if errResp, ok := s.require(sess, req, protocol.RolePassenger); !ok {
			return errResp
		}
		return s.opListReservations(sess, req)
	case protocol.OpCancelReservation:
		if errResp, ok := s.require(sess, req, protocol.RolePassenger); !ok {
			return errResp
		}
		return s.opCancelReservation(sess, req)
	default:
		return protocol.Error(req.RequestID, protocol.CodeUnknownOperation, "unknown operation "+req.Operation)
	}
}

func (s *Server) require(sess *Session, req *protocol.Request, role string) (protocol.Response, bool) {
	if !sess.Authenticated {
		return protocol.Error(req.RequestID, protocol.CodeUnauthenticated, "login required"), false
	}
	if sess.Role != role {
		return protocol.Error(req.RequestID, protocol.CodeForbidden, "operation not allowed for role "+sess.Role), false
	}
	return protocol.Response{}, true
}

func (s *Server) opRegister(req *protocol.Request) protocol.Response {
	var in protocol.RegisterData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid register payload")
	}
	u, err := s.st.Register(in.Username, in.Password, in.Role)
	if err != nil {
		return mapErr(req.RequestID, err)
	}
	return protocol.OK(req.RequestID, protocol.RegisterResult{UserID: u.UserID, Username: u.Username, Role: u.Role})
}

func (s *Server) opLogin(sess *Session, req *protocol.Request) protocol.Response {
	var in protocol.LoginData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid login payload")
	}
	u, err := s.st.Authenticate(in.Username, in.Password)
	if err != nil {
		return protocol.Error(req.RequestID, protocol.CodeForbidden, "invalid credentials")
	}
	sess.Authenticated = true
	sess.UserID = u.UserID
	sess.Username = u.Username
	sess.Role = u.Role
	return protocol.OK(req.RequestID, protocol.LoginResult{UserID: u.UserID, Username: u.Username, Role: u.Role})
}

func (s *Server) opLogout(sess *Session, req *protocol.Request) protocol.Response {
	*sess = Session{Role: "CLOSING"}
	return protocol.OK(req.RequestID, map[string]string{"message": "bye"})
}

func (s *Server) opPublishRide(sess *Session, req *protocol.Request) protocol.Response {
	var in protocol.PublishRideData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid publish payload")
	}
	ride, err := s.st.PublishRide(sess.UserID, in.Cities, in.DepartureDate, in.DepartureTime, in.Capacity, in.SegmentPrices)
	if err != nil {
		return mapErr(req.RequestID, err)
	}
	return protocol.OK(req.RequestID, service.RideView(ride))
}

func (s *Server) opListDriverRides(sess *Session, req *protocol.Request) protocol.Response {
	rides := s.st.ListDriverRides(sess.UserID)
	views := make([]protocol.RideView, 0, len(rides))
	for _, r := range rides {
		views = append(views, service.RideView(r))
	}
	return protocol.OK(req.RequestID, protocol.ListDriverRidesResult{Rides: views})
}

func (s *Server) opCancelRide(sess *Session, req *protocol.Request) protocol.Response {
	var in protocol.CancelRideData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid payload")
	}
	ride, err := s.st.CancelRide(sess.UserID, in.RideID)
	if err != nil {
		return mapErr(req.RequestID, err)
	}
	return protocol.OK(req.RequestID, service.RideView(ride))
}

func (s *Server) opListRidePassengers(sess *Session, req *protocol.Request) protocol.Response {
	var in protocol.ListRidePassengersData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid payload")
	}
	ride, seats, err := s.st.RidePassengers(sess.UserID, in.RideID)
	if err != nil {
		return mapErr(req.RequestID, err)
	}
	out := protocol.ListRidePassengersResult{RideID: ride.RideID}
	for i, seg := range ride.Segments {
		pv := protocol.SegmentPassengersView{
			SegmentIndex: seg.SegmentIndex,
			Origin:       seg.Origin,
			Destination:  seg.Destination,
		}
		if i < len(seats) {
			for _, p := range seats[i] {
				pv.Passengers = append(pv.Passengers, protocol.PassengerOnLeg{
					UserID: p.UserID, Username: p.Username, ReservationID: p.ReservationID,
				})
			}
		}
		if pv.Passengers == nil {
			pv.Passengers = []protocol.PassengerOnLeg{}
		}
		out.Segments = append(out.Segments, pv)
	}
	return protocol.OK(req.RequestID, out)
}

func (s *Server) opSearch(_ *Session, req *protocol.Request) protocol.Response {
	var in protocol.SearchItinerariesData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid search payload")
	}
	its, err := s.svc.Search(in.Origin, in.Destination, in.Date)
	if err != nil {
		return mapErr(req.RequestID, err)
	}
	views := make([]protocol.ItineraryView, 0, len(its))
	for _, it := range its {
		views = append(views, service.ItineraryView(it))
	}
	return protocol.OK(req.RequestID, protocol.SearchItinerariesResult{Itineraries: views})
}

func (s *Server) opConfirm(sess *Session, req *protocol.Request) protocol.Response {
	var in protocol.ConfirmReservationData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid confirm payload")
	}
	inputs := make([]domain.LegInput, 0, len(in.Legs))
	for _, l := range in.Legs {
		inputs = append(inputs, domain.LegInput{RideID: l.RideID, Origin: l.Origin, Destination: l.Destination})
	}
	res, err := s.st.ConfirmReservation(sess.UserID, req.RequestID, inputs)
	if err != nil {
		return mapErr(req.RequestID, err)
	}
	return protocol.OK(req.RequestID, protocol.ConfirmReservationResult{Reservation: service.ReservationView(res)})
}

func (s *Server) opListReservations(sess *Session, req *protocol.Request) protocol.Response {
	list := s.st.ListReservations(sess.UserID)
	views := make([]protocol.ReservationView, 0, len(list))
	for _, r := range list {
		views = append(views, service.ReservationView(r))
	}
	return protocol.OK(req.RequestID, protocol.ListReservationsResult{Reservations: views})
}

func (s *Server) opCancelReservation(sess *Session, req *protocol.Request) protocol.Response {
	var in protocol.CancelReservationData
	if err := json.Unmarshal(nonzero(req.Data), &in); err != nil {
		return protocol.Error(req.RequestID, protocol.CodeValidationError, "invalid payload")
	}
	res, err := s.st.CancelReservation(sess.UserID, in.ReservationID)
	if err != nil {
		return mapErr(req.RequestID, err)
	}
	return protocol.OK(req.RequestID, protocol.CancelReservationResult{Reservation: service.ReservationView(res)})
}

func mapErr(requestID string, err error) protocol.Response {
	switch {
	case errors.Is(err, store.ErrNoSeats):
		return protocol.Error(requestID, protocol.CodeNoSeats, "one or more segments have no seats")
	case errors.Is(err, store.ErrNotFound):
		return protocol.Error(requestID, protocol.CodeNotFound, err.Error())
	case errors.Is(err, store.ErrForbidden):
		return protocol.Error(requestID, protocol.CodeForbidden, err.Error())
	case errors.Is(err, store.ErrValidation):
		return protocol.Error(requestID, protocol.CodeValidationError, err.Error())
	case errors.Is(err, store.ErrAlreadyExists):
		return protocol.Error(requestID, protocol.CodeValidationError, "username already exists")
	case errors.Is(err, store.ErrAlreadyCancelled):
		return protocol.Error(requestID, protocol.CodeAlreadyCancelled, err.Error())
	case errors.Is(err, store.ErrConflict):
		return protocol.Error(requestID, protocol.CodeReservationConflict, err.Error())
	default:
		return protocol.Error(requestID, protocol.CodeInternalError, fmt.Sprintf("internal error: %v", err))
	}
}

func nonzero(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}
