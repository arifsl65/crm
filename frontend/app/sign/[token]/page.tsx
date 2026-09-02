import SigningClient from './SigningClient';

// Required for static export: generates a catch-all fallback page.
// Actual tokens are resolved client-side after hydration.
export async function generateStaticParams() {
  return [{ token: 'placeholder' }];
}

export default function SigningPage({ params }: { params: { token: string } }) {
  return <SigningClient token={params.token} />;
}
