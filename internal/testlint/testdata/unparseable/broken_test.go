// Deliberately unparseable: testlint must report a file it cannot read rather
// than skip it and pass vacuously (reach_test.go TestScanReportsAnUnparseableFile).
package broken

func TestBroken(t *testing.T) {
