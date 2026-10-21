import { describe, expect, it } from "vitest";
import { remoteImagesNote } from "./remote";

const meta = (remoteImages: number, remoteImagesLoaded: number) => ({ remoteImages, remoteImagesLoaded });

describe("remoteImagesNote", () => {
  it("says nothing when the diagram has no remote images", () => {
    expect(remoteImagesNote(meta(0, 0), false)).toBeNull();
    expect(remoteImagesNote(meta(0, 0), true)).toBeNull();
    expect(remoteImagesNote(null, false)).toBeNull();
  });

  it("says what is hidden, and offers to show it, when remote images are off", () => {
    expect(remoteImagesNote(meta(1, 0), false)).toEqual({ text: "1 remote image hidden", canShow: true });
    expect(remoteImagesNote(meta(3, 0), false)).toEqual({ text: "3 remote images hidden", canShow: true });
  });

  it("is quiet when they are on and all loaded", () => {
    expect(remoteImagesNote(meta(2, 2), true)).toBeNull();
  });

  it("reports failures when they are on", () => {
    expect(remoteImagesNote(meta(3, 1), true)).toEqual({
      text: "2 of 3 remote images could not be loaded",
      canShow: false,
    });
    expect(remoteImagesNote(meta(1, 0), true)).toEqual({
      text: "1 of 1 remote image could not be loaded",
      canShow: false,
    });
  });
});
