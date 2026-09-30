/**
 * APIPulse Mock Data — Vendors
 * DEMO DATA ONLY — Not real credentials or integrations
 */
import type { Vendor } from '../../types/domain';

export const MOCK_VENDORS: Vendor[] = [
  {
    id: 'stripe',
    name: 'stripe',
    displayName: 'Stripe',
    category: 'payment',
    statusPageUrl: 'https://status.stripe.com',
    vendorReportedStatus: 'operational',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 5,
  },
  {
    id: 'twilio',
    name: 'twilio',
    displayName: 'Twilio',
    category: 'communication',
    statusPageUrl: 'https://status.twilio.com',
    vendorReportedStatus: 'operational',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 10,
  },
  {
    id: 'auth0',
    name: 'auth0',
    displayName: 'Auth0',
    category: 'auth',
    statusPageUrl: 'https://status.auth0.com',
    vendorReportedStatus: 'operational',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 8,
  },
  {
    id: 'sendgrid',
    name: 'sendgrid',
    displayName: 'SendGrid',
    category: 'communication',
    statusPageUrl: 'https://status.sendgrid.com',
    vendorReportedStatus: 'degraded_performance',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 22,
  },
  {
    id: 'aws-s3',
    name: 'aws-s3',
    displayName: 'AWS S3',
    category: 'storage',
    statusPageUrl: 'https://health.aws.amazon.com',
    vendorReportedStatus: 'operational',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 3,
  },
  {
    id: 'cloudflare',
    name: 'cloudflare',
    displayName: 'Cloudflare',
    category: 'cdn',
    statusPageUrl: 'https://www.cloudflarestatus.com',
    vendorReportedStatus: 'operational',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 7,
  },
  {
    id: 'openai',
    name: 'openai',
    displayName: 'OpenAI',
    category: 'ml',
    statusPageUrl: 'https://status.openai.com',
    vendorReportedStatus: 'operational',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 15,
  },
  {
    id: 'datadog',
    name: 'datadog',
    displayName: 'Datadog',
    category: 'analytics',
    statusPageUrl: 'https://status.datadoghq.com',
    vendorReportedStatus: 'operational',
    vendorStatusUpdatedAt: Date.now() - 1000 * 60 * 4,
  },
];

export const getVendorById = (id: string): Vendor | undefined =>
  MOCK_VENDORS.find(v => v.id === id);
