/** Display text for the API-stamped conversion state. Does not decide the state. */
export function conversionStateLabel(state: string | undefined): string {
  switch (state) {
    case 'converted':
      return 'Converted';
    case 'converted_zero':
      return 'Converted · $0';
    case 'not_converted':
      return 'Not converted';
    default:
      return 'n/a';
  }
}
