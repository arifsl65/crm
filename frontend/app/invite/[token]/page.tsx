import InviteAcceptForm from './InviteAcceptForm';

// Required for static export: generates a catch-all fallback page.
// Actual token is resolved client-side after hydration.
export async function generateStaticParams() {
  return [{ token: 'placeholder' }];
}

export default function InviteAcceptPage({ params }: { params: { token: string } }) {
  return <InviteAcceptForm token={params.token} />;
}
