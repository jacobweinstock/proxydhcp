package tink

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/credentials"
)

var fakeCert = []byte(`-----BEGIN CERTIFICATE-----
MIIEMDCCAxigAwIBAgIQUJRs7Bjq1ZxN1ZfvdY+grTANBgkqhkiG9w0BAQUFADCB
gjELMAkGA1UEBhMCVVMxHjAcBgNVBAsTFXd3dy54cmFtcHNlY3VyaXR5LmNvbTEk
MCIGA1UEChMbWFJhbXAgU2VjdXJpdHkgU2VydmljZXMgSW5jMS0wKwYDVQQDEyRY
UmFtcCBHbG9iYWwgQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwHhcNMDQxMTAxMTcx
NDA0WhcNMzUwMTAxMDUzNzE5WjCBgjELMAkGA1UEBhMCVVMxHjAcBgNVBAsTFXd3
dy54cmFtcHNlY3VyaXR5LmNvbTEkMCIGA1UEChMbWFJhbXAgU2VjdXJpdHkgU2Vy
dmljZXMgSW5jMS0wKwYDVQQDEyRYUmFtcCBHbG9iYWwgQ2VydGlmaWNhdGlvbiBB
dXRob3JpdHkwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCYJB69FbS6
38eMpSe2OAtp87ZOqCwuIR1cRN8hXX4jdP5efrRKt6atH67gBhbim1vZZ3RrXYCP
KZ2GG9mcDZhtdhAoWORlsH9KmHmf4MMxfoArtYzAQDsRhtDLooY2YKTVMIJt2W7Q
DxIEM5dfT2Fa8OT5kavnHTu86M/0ay00fOJIYRyO82FEzG+gSqmUsE3a56k0enI4
qEHMPJQRfevIpoy3hsvKMzvZPTeL+3o+hiznc9cKV6xkmxnr9A8ECIqsAxcZZPRa
JSKNNCyy9mgdEm3Tih4U2sSPpuIjhdV6Db1q4Ons7Be7QhtnqiXtRYMh/MHJfNVi
PvryxS3T/dRlAgMBAAGjgZ8wgZwwEwYJKwYBBAGCNxQCBAYeBABDAEEwCwYDVR0P
BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFMZPoj0GY4QJnM5i5ASs
jVy16bYbMDYGA1UdHwQvMC0wK6ApoCeGJWh0dHA6Ly9jcmwueHJhbXBzZWN1cml0
eS5jb20vWEdDQS5jcmwwEAYJKwYBBAGCNxUBBAMCAQEwDQYJKoZIhvcNAQEFBQAD
ggEBAJEVOQMBG2f7Shz5CmBbodpNl2L5JFMn14JkTpAuw0kbK5rc/Kh4ZzXxHfAR
vbdI4xD2Dd8/0sm2qlWkSLoC295ZLhVbO50WfUfXN+pfTXYSNrsf16GBBEYgoyxt
qZ4Bfj8pzgCT3/3JknOJiWSe5yvkHJEs0rnOfc5vMZnT5r7SHpDwCRR5XCOrTdLa
IR9NmXmd4c8nnxCbHIgNsIpkQTG4DmyQJKSbXHGPurt+HBvbaoAPIbzp26a3QPSy
i6mx5O+aGtA9aZnuqCij4Tyz8LIRnM98QObd50N9otg6tamN8jSZxNQQ4Qb9CYQQ
O+7ETPTsJ3xCwnR8gooJybQDJbw=
-----END CERTIFICATE-----`)

func TestToCreds(t *testing.T) {
	tests := map[string]struct {
		cert []byte
		want *credentials.ProtocolInfo
	}{
		"failed to parse": {cert: []byte{}, want: nil},
		"success": {cert: fakeCert, want: &credentials.ProtocolInfo{
			SecurityProtocol: "tls",
			SecurityVersion:  "1.2",
		}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := toCreds(tc.cert)
			if got == nil {
				if tc.want != nil {
					t.Fatalf("wanted nil, got: %v", got)
				}
			} else {
				if diff := cmp.Diff(*tc.want, got.Info()); diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}
