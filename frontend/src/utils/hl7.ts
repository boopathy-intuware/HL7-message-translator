// Splits raw HL7v2 text into one entry per segment, mirroring the segment
// split in backend/hl7 (segments separated by \r, with bare \n also
// tolerated for hand-typed fixtures).
export function splitHL7Segments(raw: string): string[] {
  return raw.split(/\r\n|\r|\n/).filter((segment) => segment.length > 0)
}
