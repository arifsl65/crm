import DocumentDetailClient from './DocumentDetailClient';

// Required for static export: generates a catch-all fallback page.
// Actual document IDs are resolved client-side after hydration.
export async function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

export default function DocumentDetailPage({ params }: { params: { id: string } }) {
  return <DocumentDetailClient documentId={params.id} />;
}
