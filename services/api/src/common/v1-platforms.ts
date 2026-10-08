/** v1 "All" counts Ringba and Retreaver only. Other platforms stay in the table. */
export const V1_PLATFORM_SQL = "AND platform IN ('ringba', 'retreaver')";

export function platformScopeSql(
  platform?: string,
  v1Platforms?: boolean,
): { sql: string; params: Record<string, string> } {
  if (platform) {
    return { sql: 'AND platform = {platform:String}', params: { platform } };
  }
  if (v1Platforms) {
    return { sql: V1_PLATFORM_SQL, params: {} };
  }
  return { sql: '', params: {} };
}
