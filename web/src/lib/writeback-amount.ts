/** True when the operator may confirm a convert. Finite and greater than zero; no upper cap. */
export function isConfirmableConvertAmount(amount: number | null): amount is number {
  return amount !== null && Number.isFinite(amount) && amount > 0;
}
