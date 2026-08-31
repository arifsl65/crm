import ClientDetail from './ClientDetail';

// Required for static export: generates a catch-all fallback page.
// Actual client IDs are resolved client-side after hydration.
export async function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

export default function ClientDetailPage({ params }: { params: { id: string } }) {
  return <ClientDetail clientId={params.id} />;
}
