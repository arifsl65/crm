import type { Metadata, Viewport } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';
import { ThemeProvider } from '@/components/theme-provider';

const inter = Inter({
  subsets: ['latin'],
  display: 'swap',
  variable: '--font-inter',
});

export const metadata: Metadata = {
  title: {
    default: 'Accountant CRM',
    template: '%s | Accountant CRM',
  },
  description: 'AI-powered accounting and CRM platform for modern businesses',
  keywords: ['accounting', 'CRM', 'AI', 'invoicing', 'document processing'],
  authors: [{ name: 'Accountant CRM Team' }],
  creator: 'Accountant CRM',
  metadataBase: new URL('https://accountant-crm.com'),
  openGraph: {
    type: 'website',
    locale: 'en_US',
    url: 'https://accountant-crm.com',
    siteName: 'Accountant CRM',
    title: 'Accountant CRM',
    description: 'AI-powered accounting and CRM platform for modern businesses',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Accountant CRM',
    description: 'AI-powered accounting and CRM platform for modern businesses',
  },
  robots: {
    index: true,
    follow: true,
  },
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 1,
  themeColor: [
    { media: '(prefers-color-scheme: light)', color: '#ffffff' },
    { media: '(prefers-color-scheme: dark)', color: '#0f172a' },
  ],
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className={`${inter.variable} font-sans antialiased`}>
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          {children}
        </ThemeProvider>
      </body>
    </html>
  );
}
