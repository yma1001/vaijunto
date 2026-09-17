package cliui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yma1001/vaijunto/internal/protocol"
)

// TestParseBRDate: DD/MM/AAAA vira YYYY-MM-DD; 31/02 é rejeitado.
func TestParseBRDate(t *testing.T) {
	iso, err := ParseBRDate("16/09/2026")
	if err != nil || iso != "2026-09-16" {
		t.Fatalf("got %q %v", iso, err)
	}
	if _, err := ParseBRDate("31/02/2026"); err == nil {
		t.Fatal("31/02 must fail")
	}
	if _, err := ParseBRDate("2026-09-16"); err == nil {
		t.Fatal("ISO must not be accepted in the CLI")
	}
	if _, err := ParseBRDate(""); err == nil {
		t.Fatal("empty date")
	}
	if _, err := ParseBRDate("aa/bb/cccc"); err == nil {
		t.Fatal("garbage date")
	}
}

// TestFormatBRDate: YYYY-MM-DD vira DD/MM/AAAA na tela.
func TestFormatBRDate(t *testing.T) {
	if got := FormatBRDate("2026-10-10"); got != "10/10/2026" {
		t.Fatalf("got %q", got)
	}
}

// TestParseBRLToCents: 15, 15,50 e 15.50 viram centavos sem float64.
func TestParseBRLToCents(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"15", 1500},
		{"15,50", 1550},
		{"15.50", 1550},
		{"15,5", 1550},
		{"0", 0},
		{"0,01", 1},
		{"100", 10000},
	}
	for _, c := range cases {
		got, err := ParseBRLToCents(c.in)
		if err != nil || got != c.want {
			t.Fatalf("%q: got %d %v want %d", c.in, got, err, c.want)
		}
	}
}

// TestParseBRLToCentsInvalid: lista de preços ou texto solto não é aceito.
func TestParseBRLToCentsInvalid(t *testing.T) {
	bads := []string{
		"",
		"-15",
		"+15",
		"15,555",
		"15,50,20",
		"abc",
		"15,5.0",
		"1.500,00",
		"15,50.00",
		",50",
		"15,",
		"15.505",
	}
	for _, in := range bads {
		if _, err := ParseBRLToCents(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

// TestFormatBRL: 1500 centavos aparecem como R$ 15,00.
func TestFormatBRL(t *testing.T) {
	if got := FormatBRL(1500); got != "R$ 15,00" {
		t.Fatalf("got %q", got)
	}
	if got := FormatBRL(1550); got != "R$ 15,50" {
		t.Fatalf("got %q", got)
	}
	if got := FormatBRL(1); got != "R$ 0,01" {
		t.Fatalf("got %q", got)
	}
}

// TestParseCitiesTwoAndFour: a rota aceita 2 ou 4+ cidades, sem teto de 3.
func TestParseCitiesTwoAndFour(t *testing.T) {
	two, err := ParseCities("Salvador, Feira de Santana")
	if err != nil || len(two) != 2 {
		t.Fatalf("%v %#v", err, two)
	}
	four, err := ParseCities("Salvador, Feira de Santana, Jequié, Vitória da Conquista")
	if err != nil || len(four) != 4 {
		t.Fatalf("%v %#v", err, four)
	}
	if four[3] != "Vitória da Conquista" {
		t.Fatalf("trim/keep names: %#v", four)
	}
}

// TestParseCitiesRejectsEmptyAndShort: uma cidade só, ou item vazio entre vírgulas, é erro.
func TestParseCitiesRejectsEmptyAndShort(t *testing.T) {
	if _, err := ParseCities("Salvador"); err == nil {
		t.Fatal("one city")
	}
	if _, err := ParseCities("Salvador, , Feira de Santana"); err == nil {
		t.Fatal("empty item")
	}
	if _, err := ParseCities(""); err == nil {
		t.Fatal("empty")
	}
}

// TestStatusPT: ACTIVE/CANCELLED/CONFIRMED viram ATIVA/CANCELADA/CONFIRMADA na CLI.
func TestStatusPT(t *testing.T) {
	if StatusPT("ACTIVE") != "ATIVA" {
		t.Fatal(StatusPT("ACTIVE"))
	}
	if StatusPT("CANCELLED") != "CANCELADA" {
		t.Fatal(StatusPT("CANCELLED"))
	}
	if StatusPT("CONFIRMED") != "CONFIRMADA" {
		t.Fatal(StatusPT("CONFIRMED"))
	}
}

// TestFormatRideAndItineraryNoRawJSON: a CLI não imprime o JSON cru do protocolo.
func TestFormatRideAndItineraryNoRawJSON(t *testing.T) {
	ride := protocol.RideView{
		RideID:        "ride-abc",
		Cities:        []string{"Salvador", "Feira de Santana"},
		DepartureDate: "2026-10-10",
		DepartureTime: "08:00",
		Capacity:      2,
		Status:        "ACTIVE",
		Segments: []protocol.SegmentView{
			{Origin: "Salvador", Destination: "Feira de Santana", Price: 1550, AvailableSeats: 2},
		},
	}
	pass := protocol.ListRidePassengersResult{
		RideID: "ride-abc",
		Segments: []protocol.SegmentPassengersView{
			{
				Origin: "Salvador", Destination: "Feira de Santana",
				Passengers: []protocol.PassengerOnLeg{{Username: "ana", ReservationID: "res-1"}},
			},
		},
	}
	out := FormatRide(1, ride, &pass)
	for _, need := range []string{"ID: ride-abc", "ATIVA", "Salvador → Feira de Santana", "10/10/2026", "R$ 15,50", "ana", "res-1"} {
		if !strings.Contains(out, need) {
			t.Fatalf("missing %q in\n%s", need, out)
		}
	}
	if strings.Contains(out, `"rideId"`) || strings.Contains(out, `"status"`) {
		t.Fatalf("raw JSON leaked:\n%s", out)
	}

	it := protocol.ItineraryView{
		TotalPrice: 1550,
		Transfers:  0,
		Legs: []protocol.LegView{
			{
				RideID: "ride-abc", Origin: "Salvador", Destination: "Feira de Santana",
				DepartureDate: "2026-10-10", DepartureTime: "08:00", Price: 1550, AvailableSeats: 2,
			},
		},
	}
	itOut := FormatItinerary(1, it)
	for _, need := range []string{"Origem: Salvador", "Destino: Feira de Santana", "R$ 15,50", "Baldeações: 0", "Carona: ride-abc"} {
		if !strings.Contains(itOut, need) {
			t.Fatalf("missing %q in\n%s", need, itOut)
		}
	}

	res := protocol.ReservationView{
		ReservationID: "res-1",
		Status:        "CONFIRMED",
		TotalPrice:    1550,
		CreatedAt:     "2026-09-16T12:30:00Z",
		Legs:          it.Legs,
	}
	rOut := FormatReservation(1, res)
	for _, need := range []string{"ID: res-1", "CONFIRMADA", "16/09/2026 12:30", "R$ 15,50"} {
		if !strings.Contains(rOut, need) {
			t.Fatalf("missing %q in\n%s", need, rOut)
		}
	}
}

// TestValidateRegisterInput: senha e confirmação precisam bater antes de chamar o servidor.
func TestValidateRegisterInput(t *testing.T) {
	if err := ValidateRegisterInput("", "a", "a"); err == nil {
		t.Fatal("empty user")
	}
	if err := ValidateRegisterInput("u", "", ""); err == nil {
		t.Fatal("empty pass")
	}
	if err := ValidateRegisterInput("u", "a", "b"); err == nil {
		t.Fatal("mismatch")
	}
	if err := ValidateRegisterInput("u", "a", "a"); err != nil {
		t.Fatal(err)
	}
}

// TestFriendlyError: credencial inválida e conexão recusada viram texto em português.
func TestFriendlyError(t *testing.T) {
	if got := FriendlyError(errString("REGISTER failed: VALIDATION_ERROR username already exists")); got != "este usuário já existe" {
		t.Fatal(got)
	}
	if got := FriendlyError(errString("dial tcp 127.0.0.1:5000: connection refused")); got != "falha de comunicação com o servidor" {
		t.Fatal(got)
	}
}

// TestNormalizeLineStripsCRLFSpaceAndNUL: stdin Windows (CR/NUL) não quebra o menu.
func TestNormalizeLineStripsCRLFSpaceAndNUL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"6", "6"},
		{" 6 ", "6"},
		{"6\r", "6"},
		{"6\x00", "6"},
		{"6\x00\r", "6"},
		{"\x006\x00", "6"},
	}
	for _, c := range cases {
		if got := NormalizeLine(c.in); got != c.want {
			t.Fatalf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}

// TestPrompterAcceptsLFCRLFAndLoneCR: o menu funciona igual no Linux e no Windows.
func TestPrompterAcceptsLFCRLFAndLoneCR(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("6\r\n 7 \n8\x00\n9\r10\n"), &out)
	got := []string{p.Read(""), p.Read(""), p.Read(""), p.Read(""), p.Read("")}
	want := []string{"6", "7", "8", "9", "10"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %q want %q (%v)", i, got[i], want[i], got)
		}
	}
	if p.EOF {
		t.Fatal("should not be EOF yet")
	}
	if p.Read("") != "" || !p.EOF {
		t.Fatal("expected EOF after last line")
	}
}

// TestPrompterCRLFIsSingleToken: \r\n conta como uma linha, não duas.
func TestPrompterCRLFIsSingleToken(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("6\r\n"), &out)
	if got := p.Read("> "); got != "6" {
		t.Fatalf("got %q", got)
	}
	if p.Read("> ") != "" || !p.EOF {
		t.Fatal("CRLF must not produce an extra empty line")
	}
	if !strings.Contains(out.String(), "> ") {
		t.Fatal("prompt should be written")
	}
}

// TestResolveListChoice: o passageiro confirma pelo número 1-based da última busca.
func TestResolveListChoice(t *testing.T) {
	ids := []string{"ride-a", "ride-b"}
	i, err := ResolveListChoice("1", ids)
	if err != nil || i != 0 {
		t.Fatalf("%d %v", i, err)
	}
	i, err = ResolveListChoice("ride-b", ids)
	if err != nil || i != 1 {
		t.Fatalf("%d %v", i, err)
	}
	if _, err := ResolveListChoice("9", ids); err == nil {
		t.Fatal("oob")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
