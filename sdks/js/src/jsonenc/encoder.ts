/** Encode a payload to JSON string. */
export function encodeJSON(payload: Record<string, any>): string {
  return JSON.stringify(payload);
}

/** Encode a payload to pretty-printed JSON string. */
export function encodePrettyJSON(payload: Record<string, any>): string {
  return JSON.stringify(payload, null, 2);
}
