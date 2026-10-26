package factures

import (
	"fmt"
	"os"
)

// Exporter écrit les lignes de la facture dans un fichier CSV : libellé,
// prix unitaire en euros, quantité.
func Exporter(f Facture, chemin string) error {
	fichier, err := os.Create(chemin)
	if err != nil {
		return err
	}
	fmt.Fprintln(fichier, "libelle,prix,quantite")
	for _, l := range f.Lignes {
		prix := float64(l.PrixCentimes) / 100
		fmt.Fprintf(fichier, "%s,%.2f,%d\n", l.Libelle, prix, l.Quantite)
	}
	return nil
}
