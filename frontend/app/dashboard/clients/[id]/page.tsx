import ClientDetail from './ClientDetail';

// Required for static export: generates a catch-all fallback page.
// Actual client IDs are resolved client-side via useParams().
export async function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

export default function ClientDetailPage() {
  return <ClientDetail />;
}
