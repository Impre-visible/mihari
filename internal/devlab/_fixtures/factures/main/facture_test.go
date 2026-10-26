package factures

import "testing"

func TestTotalHT(t *testing.T) {
	f := Facture{Lignes: []Ligne{{"Stylo", 150, 4}, {"Cahier", 320, 2}}}
	if got := f.TotalHT(); got != 1240 {
		t.Errorf("TotalHT = %d, attendu 1240", got)
	}
	f.RemisePourcent = 10
	if got := f.TotalHT(); got != 1116 {
		t.Errorf("TotalHT avec 10 %% de remise = %d, attendu 1116", got)
	}
}
