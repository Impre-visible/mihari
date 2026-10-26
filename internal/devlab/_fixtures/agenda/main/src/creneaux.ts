export type Creneau = { debut: Date; fin: Date };

const MINUTE = 60_000;

// chevauche dit si deux créneaux ont un instant en commun ; deux créneaux qui
// se touchent ne se chevauchent pas.
export function chevauche(a: Creneau, b: Creneau): boolean {
  return a.debut < b.fin && b.debut < a.fin;
}

// libres renvoie, dans l'ordre, les créneaux de dureeMinutes qui tiennent entre
// debut et fin sans chevaucher aucun rendez-vous.
export function libres(rdv: Creneau[], debut: Date, fin: Date, dureeMinutes: number): Creneau[] {
  const duree = dureeMinutes * MINUTE;
  const pris = [...rdv].sort((a, b) => a.debut.getTime() - b.debut.getTime());
  const out: Creneau[] = [];
  let curseur = debut.getTime();
  for (const r of pris) {
    while (curseur + duree <= Math.min(r.debut.getTime(), fin.getTime())) {
      out.push({ debut: new Date(curseur), fin: new Date(curseur + duree) });
      curseur += duree;
    }
    curseur = Math.max(curseur, r.fin.getTime());
  }
  while (curseur + duree <= fin.getTime()) {
    out.push({ debut: new Date(curseur), fin: new Date(curseur + duree) });
    curseur += duree;
  }
  return out;
}
