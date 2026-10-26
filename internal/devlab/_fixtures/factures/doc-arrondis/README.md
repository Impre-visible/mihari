# factures

Calcul des totaux d'une facture : total HT, remise et TVA. Les montants sont
en centimes, pour éviter les erreurs d'arrondi des flottants.

## Arrondi de la TVA

La TVA est calculée sur le total HT de la facture, pas ligne par ligne, puis
arrondie au centime le plus proche (0,5 centime arrondi au-dessus).

```
go test ./...
```
