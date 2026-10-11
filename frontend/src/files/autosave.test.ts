import { describe, expect, it } from "vitest";
import { AUTOSAVE_CHOICES, currentChoice } from "./autosave";

describe("autosave choices", () => {
  it("starts with Off, the default", () => {
    expect(AUTOSAVE_CHOICES[0]).toMatchObject({ id: "off", mode: "off" });
  });

  it("matches saved settings to the preset", () => {
    expect(currentChoice("off", 2).id).toBe("off");
    expect(currentChoice("delay", 2).id).toBe("delay-2");
    expect(currentChoice("delay", 10).id).toBe("delay-10");
    expect(currentChoice("blur", 2).id).toBe("blur");
    expect(currentChoice("blur", 99).id).toBe("blur"); // the delay is irrelevant in blur mode
  });

  it("describes a custom delay accurately", () => {
    expect(currentChoice("delay", 30)).toMatchObject({ mode: "delay", delaySec: 30, label: "After a 30-second pause" });
  });

  it("treats anything unknown as off, never as on", () => {
    expect(currentChoice("", 2).id).toBe("off");
    expect(currentChoice("always", 2).id).toBe("off");
  });
});
