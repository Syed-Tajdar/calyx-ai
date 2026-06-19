export function usePathname(): string {
  return '/intelligence';
}

export function useRouter(): { push: () => void; replace: () => void } {
  return { push: () => undefined, replace: () => undefined };
}

export function useSearchParams(): URLSearchParams {
  return new URLSearchParams();
}

export function useParams(): Record<string, string> {
  return {};
}
