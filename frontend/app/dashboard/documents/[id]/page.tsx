import DocumentDetail from './DocumentDetail';

// Required for static export: generates a catch-all fallback page.
// Actual document IDs are resolved client-side via useParams().
export async function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

export default function DocumentDetailPage() {
  return <DocumentDetail />;
}
