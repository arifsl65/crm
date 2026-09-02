'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { getSigningPageData, submitSignature, SigningPageData } from '@/lib/api';

interface SigningClientProps {
  token: string;
}

export default function SigningClient({ token: propToken }: SigningClientProps) {
  const [actualToken, setActualToken] = useState<string | null>(null);
  const [pageData, setPageData] = useState<SigningPageData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [errorType, setErrorType] = useState<'invalid' | 'expired' | 'signed' | 'declined' | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [success, setSuccess] = useState(false);
  const [signedAt, setSignedAt] = useState<string | null>(null);

  // Form state
  const [fullName, setFullName] = useState('');
  const [agreedToTerms, setAgreedToTerms] = useState(false);
  const [signature, setSignature] = useState('');
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [isDrawing, setIsDrawing] = useState(false);
  const [hasSignature, setHasSignature] = useState(false);

  // Extract actual token from URL on mount
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const pathParts = window.location.pathname.split('/');
      const urlToken = pathParts[pathParts.length - 1];
      setActualToken(propToken === 'placeholder' || !propToken ? urlToken : propToken);
    }
  }, [propToken]);

  const loadSigningPage = useCallback(async (token: string) => {
    try {
      setLoading(true);
      const data = await getSigningPageData(token);
      setPageData(data);
      if (data.signer_name) {
        setFullName(data.signer_name);
      }
      setError(null);
      setErrorType(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      if (message === 'already_signed') {
        setErrorType('signed');
        setError('This document has already been signed.');
      } else if (message === 'expired') {
        setErrorType('expired');
        setError('This signing link has expired.');
      } else if (message === 'declined') {
        setErrorType('declined');
        setError('This document signing was declined.');
      } else {
        setErrorType('invalid');
        setError('This signing link is invalid or has expired.');
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (actualToken && actualToken !== 'placeholder') {
      loadSigningPage(actualToken);
    }
  }, [actualToken, loadSigningPage]);

  // Canvas drawing functions
  const getCoordinates = (e: React.MouseEvent<HTMLCanvasElement> | React.TouchEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };

    const rect = canvas.getBoundingClientRect();
    if ('touches' in e) {
      return {
        x: e.touches[0].clientX - rect.left,
        y: e.touches[0].clientY - rect.top,
      };
    }
    return {
      x: e.clientX - rect.left,
      y: e.clientY - rect.top,
    };
  };

  const startDrawing = (e: React.MouseEvent<HTMLCanvasElement> | React.TouchEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext('2d');
    if (!ctx) return;

    setIsDrawing(true);
    const { x, y } = getCoordinates(e);
    ctx.beginPath();
    ctx.moveTo(x, y);
  };

  const draw = (e: React.MouseEvent<HTMLCanvasElement> | React.TouchEvent<HTMLCanvasElement>) => {
    if (!isDrawing) return;
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext('2d');
    if (!ctx) return;

    const { x, y } = getCoordinates(e);
    ctx.lineTo(x, y);
    ctx.stroke();
    setHasSignature(true);
  };

  const stopDrawing = () => {
    setIsDrawing(false);
    const canvas = canvasRef.current;
    if (canvas) {
      setSignature(canvas.toDataURL('image/png'));
    }
  };

  const clearSignature = () => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext('2d');
    if (ctx && canvas) {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      setHasSignature(false);
      setSignature('');
    }
  };

  // Initialize canvas
  useEffect(() => {
    const canvas = canvasRef.current;
    if (canvas) {
      const ctx = canvas.getContext('2d');
      if (ctx) {
        ctx.strokeStyle = '#1e40af';
        ctx.lineWidth = 2;
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
      }
    }
  }, [pageData]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!actualToken || !hasSignature || !fullName || !agreedToTerms) return;

    try {
      setSubmitting(true);
      setError(null);
      const result = await submitSignature(actualToken, {
        signature,
        full_name: fullName,
        agreed_to_terms: agreedToTerms,
      });
      setSuccess(true);
      setSignedAt(result.signed_at);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit signature');
    } finally {
      setSubmitting(false);
    }
  };

  const getTemplateContent = (templateType: string, clientName: string, tenantName: string) => {
    switch (templateType) {
      case 'engagement':
        return {
          title: 'Engagement Letter',
          content: `This Engagement Letter confirms that ${tenantName} will provide accounting and advisory services to ${clientName}. By signing below, you agree to engage our firm for the services discussed and accept our standard terms of engagement including fee arrangements, scope of work, and mutual responsibilities.`,
        };
      case 'service_agreement':
        return {
          title: 'Service Agreement',
          content: `This Service Agreement outlines the terms and conditions under which ${tenantName} will provide professional services to ${clientName}. The agreement covers the scope of services, payment terms, confidentiality obligations, and termination provisions.`,
        };
      case 'gdpr_consent':
        return {
          title: 'GDPR Consent Form',
          content: `In accordance with the General Data Protection Regulation (GDPR), ${tenantName} requires your consent to process your personal data. By signing below, you consent to the collection, storage, and processing of your personal information for the purpose of providing accounting services to ${clientName}.`,
        };
      default:
        return {
          title: 'Document',
          content: `Please review and sign this document from ${tenantName} for ${clientName}.`,
        };
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
          <p className="mt-4 text-gray-600">Loading document...</p>
        </div>
      </div>
    );
  }

  if (error && !pageData) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
        <div className="max-w-md w-full bg-white rounded-lg shadow-lg p-8 text-center">
          <div className={`w-16 h-16 mx-auto mb-4 rounded-full flex items-center justify-center ${
            errorType === 'signed' ? 'bg-green-100' : 'bg-red-100'
          }`}>
            {errorType === 'signed' ? (
              <svg className="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            ) : (
              <svg className="w-8 h-8 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            )}
          </div>
          <h1 className="text-xl font-bold text-gray-900 mb-2">
            {errorType === 'signed' ? 'Already Signed' :
             errorType === 'expired' ? 'Link Expired' :
             errorType === 'declined' ? 'Signing Declined' : 'Invalid Link'}
          </h1>
          <p className="text-gray-600">{error}</p>
          {errorType !== 'signed' && (
            <p className="mt-4 text-sm text-gray-500">
              Please contact your accountant for assistance.
            </p>
          )}
        </div>
      </div>
    );
  }

  if (success) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center p-4">
        <div className="max-w-md w-full bg-white rounded-lg shadow-lg p-8 text-center">
          <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-green-100 flex items-center justify-center">
            <svg className="w-8 h-8 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <h1 className="text-xl font-bold text-gray-900 mb-2">Document Signed!</h1>
          <p className="text-gray-600">
            Thank you for signing the {pageData?.template_title}. A confirmation has been sent to {pageData?.signer_email}.
          </p>
          {signedAt && (
            <p className="mt-4 text-sm text-gray-500">
              Signed on {new Date(signedAt).toLocaleString()}
            </p>
          )}
        </div>
      </div>
    );
  }

  const template = pageData ? getTemplateContent(pageData.template_type, pageData.client_name, pageData.tenant_name) : null;

  return (
    <div className="min-h-screen bg-gray-50 py-8 px-4">
      <div className="max-w-2xl mx-auto">
        {/* Header */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-blue-100 flex items-center justify-center">
            <svg className="w-8 h-8 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-gray-900">{template?.title}</h1>
          <p className="mt-2 text-gray-600">
            From <span className="font-medium">{pageData?.tenant_name}</span> for <span className="font-medium">{pageData?.client_name}</span>
          </p>
        </div>

        {/* Document Content */}
        <div className="bg-white rounded-lg shadow-lg p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Document Terms</h2>
          <div className="prose prose-sm text-gray-700 bg-gray-50 p-4 rounded-md border">
            <p>{template?.content}</p>
          </div>
        </div>

        {/* Signing Form */}
        <form onSubmit={handleSubmit} className="bg-white rounded-lg shadow-lg p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Sign Document</h2>

          {error && (
            <div className="mb-4 p-4 bg-red-50 border border-red-200 rounded-md">
              <p className="text-sm text-red-700">{error}</p>
            </div>
          )}

          {/* Full Name */}
          <div className="mb-4">
            <label htmlFor="fullName" className="block text-sm font-medium text-gray-700 mb-1">
              Full Legal Name
            </label>
            <input
              type="text"
              id="fullName"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="Enter your full name"
              required
            />
          </div>

          {/* Signature Canvas */}
          <div className="mb-4">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Signature
            </label>
            <div className="border border-gray-300 rounded-md overflow-hidden">
              <canvas
                ref={canvasRef}
                width={500}
                height={150}
                className="w-full bg-white cursor-crosshair touch-none"
                onMouseDown={startDrawing}
                onMouseMove={draw}
                onMouseUp={stopDrawing}
                onMouseLeave={stopDrawing}
                onTouchStart={startDrawing}
                onTouchMove={draw}
                onTouchEnd={stopDrawing}
              />
            </div>
            <div className="flex justify-between items-center mt-2">
              <p className="text-xs text-gray-500">Draw your signature above</p>
              <button
                type="button"
                onClick={clearSignature}
                className="text-sm text-red-600 hover:text-red-800"
              >
                Clear
              </button>
            </div>
          </div>

          {/* Terms Agreement */}
          <div className="mb-6">
            <label className="flex items-start">
              <input
                type="checkbox"
                checked={agreedToTerms}
                onChange={(e) => setAgreedToTerms(e.target.checked)}
                className="mt-1 h-4 w-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
                required
              />
              <span className="ml-2 text-sm text-gray-700">
                I agree to sign this document electronically and acknowledge that my electronic signature has the same legal effect as a handwritten signature.
              </span>
            </label>
          </div>

          {/* Submit Button */}
          <button
            type="submit"
            disabled={!hasSignature || !fullName || !agreedToTerms || submitting}
            className={`w-full py-3 px-4 rounded-md text-white font-medium transition-colors ${
              !hasSignature || !fullName || !agreedToTerms || submitting
                ? 'bg-gray-400 cursor-not-allowed'
                : 'bg-blue-600 hover:bg-blue-700'
            }`}
          >
            {submitting ? (
              <span className="flex items-center justify-center">
                <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Signing...
              </span>
            ) : (
              'Sign Document'
            )}
          </button>

          {/* Expiry Notice */}
          {pageData?.expires_at && (
            <p className="mt-4 text-xs text-center text-gray-500">
              This link expires {new Date(pageData.expires_at).toLocaleString()}
            </p>
          )}
        </form>

        {/* Footer */}
        <p className="mt-6 text-center text-xs text-gray-500">
          Secure electronic signature powered by Accountant CRM
        </p>
      </div>
    </div>
  );
}
