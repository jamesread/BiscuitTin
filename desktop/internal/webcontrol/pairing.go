package webcontrol

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
)

// codeAlphabet leaves out 0, O, 1 and I, which are easy to misread on a TV.
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// codeLen characters of 32 symbols is 40 bits. Wrong codes are answered
// one at a time, a second apart, so they cannot be guessed on a LAN.
const codeLen = 8

// failDelay is how long a wrong code waits, holding up other wrong codes.
var failDelay = time.Second

// NewCode returns a random pairing code.
func NewCode() string {
	b := make([]byte, codeLen)
	_, _ = rand.Read(b) // never fails on Linux
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// FormatCode groups a code for display, e.g. "ABCD-EFGH".
func FormatCode(code string) string {
	if len(code) != codeLen {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// NormalizeCode accepts what a person types: any case, with dashes or
// spaces.
func NormalizeCode(s string) string {
	s = strings.ToUpper(s)
	return strings.NewReplacer("-", "", " ", "").Replace(s)
}

// codeHeader carries the pairing code on every RemoteService call.
const codeHeader = "X-Biscuittin-Code"

type pairing struct {
	code    string
	mu      sync.Mutex
	failing sync.Mutex
}

func (p *pairing) set(code string) {
	p.mu.Lock()
	p.code = code
	p.mu.Unlock()
}

func (p *pairing) matches(given string) bool {
	p.mu.Lock()
	want := p.code
	p.mu.Unlock()
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(NormalizeCode(given)), []byte(want)) == 1
}

// check lets a request through with the current code. Wrong codes are
// slowed down, one at a time.
func (p *pairing) check(given string) error {
	if p.matches(given) {
		return nil
	}
	p.failing.Lock()
	time.Sleep(failDelay)
	p.failing.Unlock()
	return connect.NewError(connect.CodeUnauthenticated, errors.New("pairing code needed: scan the QR code, or enter the code from Settings > Web control"))
}

func (p *pairing) interceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if err := p.check(req.Header().Get(codeHeader)); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}
