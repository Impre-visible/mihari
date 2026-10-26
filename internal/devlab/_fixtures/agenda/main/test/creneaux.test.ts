import { describe, expect, it } from "vitest";
import { chevauche } from "../src/creneaux";

const h = (heure: number) => new Date(2026, 0, 5, heure);

describe("chevauche", () => {
  it("sépare deux créneaux qui se touchent", () => {
    expect(chevauche({ debut: h(9), fin: h(10) }, { debut: h(10), fin: h(11) })).toBe(false);
  });
  it("voit un créneau inclus dans un autre", () => {
    expect(chevauche({ debut: h(9), fin: h(12) }, { debut: h(10), fin: h(11) })).toBe(true);
  });
});
