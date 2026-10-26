import type { Creneau } from "./creneaux";

// page renvoie la page numero (à partir de 1) de taille rendez-vous, triés
// par date de début.
export function page(rdv: Creneau[], numero: number, taille: number): Creneau[] {
  rdv.sort((a, b) => a.debut.getTime() - b.debut.getTime());
  const debut = numero * taille;
  return rdv.slice(debut, debut + taille);
}

// nombreDePages renvoie combien de pages il faut pour tout afficher.
export function nombreDePages(rdv: Creneau[], taille: number): number {
  return Math.floor(rdv.length / taille);
}
