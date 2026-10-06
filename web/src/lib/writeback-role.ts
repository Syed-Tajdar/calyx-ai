/** Convert and Cancel are admin-only. Any other role, including a missing role, sees nothing. */
export function canSeeWritebackActions(role: string | null | undefined): boolean {
  return role === 'admin';
}
