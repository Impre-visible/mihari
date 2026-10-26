// Package factures calcule les totaux d'une facture. Les montants sont en centimes.
package factures

import (
	"errors"
	"strconv"
	"strings"
)

type Ligne struct {
	Libelle      string
	PrixCentimes int
	Quantite     int
}

type Facture struct {
	Lignes []Ligne
	// Remise saisie à la main, en pourcent ; 0 sans remise.
	RemisePourcent int
}

// TotalHT additionne les lignes, remise déduite.
func (f Facture) TotalHT() int {
	total := 0
	for _, l := range f.Lignes {
		total += l.PrixCentimes * l.Quantite
	}
	return total - total*f.RemisePourcent/100
}

// TotalTTC ajoute la TVA au total HT, arrondie au centime le plus proche.
// tauxPourMille vaut 200 pour 20 %, 55 pour 5,5 %.
func (f Facture) TotalTTC(tauxPourMille int) int {
	ht := f.TotalHT()
	return ht + (ht*tauxPourMille+500)/1000
}

var ErrMontant = errors.New("montant invalide")

// ParseMontant lit un montant saisi en euros, « 12,50 » ou « 12.5 », en centimes.
func ParseMontant(s string) (int, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	s = strings.Replace(s, ",", ".", 1)
	euros, centimes, _ := strings.Cut(s, ".")
	if len(centimes) > 2 {
		return 0, ErrMontant
	}
	centimes = (centimes + "00")[:2]
	e, err := strconv.Atoi(euros)
	if err != nil {
		return 0, ErrMontant
	}
	c, err := strconv.Atoi(centimes)
	if err != nil {
		return 0, ErrMontant
	}
	if strings.HasPrefix(euros, "-") {
		return e*100 - c, nil
	}
	return e*100 + c, nil
}
