/** Map display sector name to route id used in mock data. */
export function sectorNameToId(name: string): string {
  const map: Record<string, string> = {
    白酒: "liquor",
    半导体: "semi",
    银行: "bank",
  };
  return map[name] ?? "liquor";
}
