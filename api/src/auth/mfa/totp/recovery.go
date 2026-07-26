package totp

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// recoveryAlphabet excludes visually ambiguous characters (0/O, 1/I/L)
// so codes are easy to transcribe by hand if needed.
const recoveryAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

// recoveryCodeLength is the number of alphabet characters per code,
// before the readability hyphen is inserted.
const recoveryCodeLength = 10

// GenerateRecoveryCodes returns n single-use recovery codes, formatted
// as two 5-character groups separated by a hyphen (e.g. "7K9QX-2MN4P").
// Callers are responsible for hashing before persisting — these are
// returned in plaintext exactly once, at generation time, the same
// as a TOTP secret's provisioning step.
func GenerateRecoveryCodes(n int) ([]string, error) {
	codes := make([]string, n)
	for i := range codes {
		code, err := randomRecoveryCode()
		if err != nil {
			return nil, fmt.Errorf("totp: generate recovery code: %w", err)
		}
		codes[i] = code
	}
	return codes, nil
}

// randomRecoveryCode draws each character via rejection sampling so
// every alphabet position is equally likely — a naive `% len(alphabet)`
// would bias toward the low end since 256 isn't a multiple of 31.
func randomRecoveryCode() (string, error) {
	var b strings.Builder
	buf := make([]byte, 1)
	for i := 0; i < recoveryCodeLength; i++ {
		if i == recoveryCodeLength/2 {
			b.WriteByte('-')
		}
		for {
			if _, err := rand.Read(buf); err != nil {
				return "", err
			}
			// 248 = len(recoveryAlphabet)*8, the largest multiple of
			// len(recoveryAlphabet) that fits in a byte — reject and
			// redraw anything at or above it to stay unbiased.
			if int(buf[0]) < len(recoveryAlphabet)*8 {
				b.WriteByte(recoveryAlphabet[int(buf[0])%len(recoveryAlphabet)])
				break
			}
		}
	}
	return b.String(), nil
}
