import StaffDetailPage from './StaffDetail';

// Required for static export: generates a catch-all fallback page.
// Actual staff IDs are resolved client-side via useParams().
export async function generateStaticParams() {
  return [{ id: 'placeholder' }];
}

export default function Page() {
  return <StaffDetailPage />;
}
