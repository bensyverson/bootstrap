package flow

import "testing"

// realPkgutil is `pkgutil --check-signature Homebrew.pkg` verbatim, run 2026-11-05 on
// the package from releases/latest/download (signed 2026-11-02) on macOS 27.
const realPkgutil = `Package "Homebrew.pkg":
   Status: signed by a developer certificate issued by Apple for distribution
   Notarization: trusted by the Apple notary service
   Signed with a trusted timestamp on: 2026-11-02 08:32:26 +0000
   Certificate Chain:
    1. Developer ID Installer: Patrick Linnane (927JGANW46)
       Expires: 2029-07-02 15:50:33 +0000
       SHA256 Fingerprint:
           3C 5D AA 66 55 A8 89 1F C1 E0 C0 60 8A EB 6B 3D 06 54 9E 05 A6 7C
           1D C9 03 CE A8 D5 21 20 AA C0
       ------------------------------------------------------------------------
    2. Developer ID Certification Authority
       Expires: 2031-09-17 00:00:00 +0000
       SHA256 Fingerprint:
           F1 6C D3 C5 4C 7F 83 CE A4 BF 1A 3E 6A 08 19 C8 AA A8 E4 A1 52 8F
           D1 44 71 5F 35 06 43 D2 DF 3A
       ------------------------------------------------------------------------
    3. Apple Root CA
       Expires: 2035-02-09 21:40:36 +0000
       SHA256 Fingerprint:
           B0 B1 73 0E CB C7 FF 45 05 14 2C 49 F1 29 5E 6E DA 6B CA ED 7E 2C
           68 C5 BE 91 B5 A1 10 01 F0 24

`

func TestHomebrewSignedReadsRealPkgutil(t *testing.T) {
	if ok, why := homebrewSigned(realPkgutil); !ok {
		t.Errorf("refused Homebrew's real package: %s", why)
	}
	// The same file, checked from inside a sandbox that can't reach the trust
	// service, came back like this (exit 1). It must be refused, not guessed at.
	if ok, _ := homebrewSigned("Package \"Homebrew.pkg\":\n   Status: invalid signature\n"); ok {
		t.Error("took an invalid signature")
	}
}
