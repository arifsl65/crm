import DocumentDetailClient from './DocumentDetailClient';

// Allow dynamic params for static export - renders any ID client-side
export const dynamicParams = true;

// Required for static export: generates a catch-all fallback page.
// Actual document IDs are resolved client-side after hydration.
export async function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

export default function DocumentDetailPage() {
  return <DocumentDetailClient />;
}
