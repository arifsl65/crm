// =============================================================================
// MongoDB Seed Data - AI Conversations
// =============================================================================
// PURPOSE: Seed AI chat conversations for E2E testing
// USAGE: mongosh "$MONGODB_URI" scripts/seed-mongo.js
// DATABASE: accountant_ai
// COLLECTION: ai_conversations
// =============================================================================

const TENANT_ID = "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11";
const ADMIN_USER = "b0000000-0000-0000-0000-000000000001";
const SARAH_USER = "b0000000-0000-0000-0000-000000000002";
const ACME_CLIENT = "e0000000-0000-0000-0000-000000000001";
const TECHSTART_CLIENT = "e0000000-0000-0000-0000-000000000002";

// Switch to the correct database
db = db.getSiblingDB("accountant_ai");

// Clean existing data for this tenant
db.ai_conversations.deleteMany({ tenant_id: TENANT_ID });
print("Cleaned existing ai_conversations for tenant: " + TENANT_ID);

// Insert sample conversations
const conversations = [
  {
    tenant_id: TENANT_ID,
    user_id: ADMIN_USER,
    client_id: ACME_CLIENT,
    title: "VAT Return Query - Acme Corporation",
    messages: [
      {
        role: "user",
        content: "What documents are missing for Acme's VAT return?",
        timestamp: new Date(Date.now() - 2 * 60 * 60 * 1000) // 2 hours ago
      },
      {
        role: "assistant",
        content: "Based on my analysis of Acme Corporation's VAT Return Q2 service, the following documents are still needed:\n\n1. **Bank Statement (Apr-Jun 2024)** - Requested 8 days ago\n2. **VAT Receipts Q2** - Requested 8 days ago\n\nThe service deadline was 3 days ago, so this is now overdue. I recommend sending a chase email to John Smith at john@acme.co.uk.",
        timestamp: new Date(Date.now() - 2 * 60 * 60 * 1000 + 5000)
      },
      {
        role: "user",
        content: "Send a chase email for the missing documents",
        timestamp: new Date(Date.now() - 1 * 60 * 60 * 1000)
      },
      {
        role: "assistant",
        content: "I've drafted a chase email for the missing VAT documents. Here's the preview:\n\n---\n**To:** john@acme.co.uk\n**Subject:** Action Required: VAT Return Documents Needed\n\nDear John,\n\nWe're still waiting for the following documents for your VAT Return Q2:\n- Bank Statement (Apr-Jun 2024)\n- VAT Receipts Q2\n\nPlease upload these at your earliest convenience through the client portal.\n\nBest regards,\nDemo Accounting Firm\n---\n\nWould you like me to send this email?",
        timestamp: new Date(Date.now() - 1 * 60 * 60 * 1000 + 5000)
      }
    ],
    context: {
      service_id: "f0000000-0000-0000-0000-000000000001",
      document_ids: ["aa000000-0000-0000-0000-000000000011", "aa000000-0000-0000-0000-000000000012"],
      action_type: "chase"
    },
    created_at: new Date(Date.now() - 2 * 60 * 60 * 1000),
    updated_at: new Date(Date.now() - 1 * 60 * 60 * 1000)
  },
  {
    tenant_id: TENANT_ID,
    user_id: SARAH_USER,
    client_id: TECHSTART_CLIENT,
    title: "R&D Tax Credits Analysis",
    messages: [
      {
        role: "user",
        content: "Can you analyze TechStart's R&D eligibility based on their uploaded documents?",
        timestamp: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000) // 5 days ago
      },
      {
        role: "assistant",
        content: "I've reviewed TechStart Ltd's documentation. Based on the R&D Report and supporting invoices, here's my analysis:\n\n**Qualifying R&D Activities:**\n- Software development for AI-powered analytics platform\n- Machine learning model training and optimization\n- API integration development\n\n**Estimated Qualifying Expenditure:** £125,000\n\n**Estimated Tax Credit:** £33,375 (26.7% rate for SME)\n\n**Key Supporting Documents:**\n- Invoice Pack 2023 (156 invoices, £284,500 total)\n- R&D Project documentation\n\nWould you like me to prepare a detailed breakdown for the HMRC submission?",
        timestamp: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000 + 10000)
      }
    ],
    context: {
      service_id: null,
      document_ids: ["aa000000-0000-0000-0000-000000000007"],
      action_type: "analysis"
    },
    created_at: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000),
    updated_at: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000)
  },
  {
    tenant_id: TENANT_ID,
    user_id: ADMIN_USER,
    client_id: null,
    title: "Dashboard Overview Help",
    messages: [
      {
        role: "user",
        content: "What's the current status of overdue services?",
        timestamp: new Date(Date.now() - 30 * 60 * 1000) // 30 mins ago
      },
      {
        role: "assistant",
        content: "Here's a summary of your overdue services:\n\n**Currently Overdue (3 services):**\n\n1. **VAT Return Q2** - Acme Corporation Ltd\n   - Deadline: 3 days ago\n   - Missing: 2 documents\n   - Assigned to: Sarah Johnson\n\n2. **CT600 Filing** - Acme Corporation Ltd\n   - Deadline: 5 days ago\n   - Missing: 1 document\n   - Assigned to: Sarah Johnson\n\n3. **Payroll Submission** - Acme Corporation Ltd\n   - Deadline: 2 days ago\n   - Missing: 1 document\n   - Assigned to: James Brown\n\n**Action Recommended:** Consider sending bulk chase emails to collect missing documents.",
        timestamp: new Date(Date.now() - 30 * 60 * 1000 + 5000)
      }
    ],
    context: {
      service_id: null,
      document_ids: [],
      action_type: "query"
    },
    created_at: new Date(Date.now() - 30 * 60 * 1000),
    updated_at: new Date(Date.now() - 30 * 60 * 1000)
  },
  {
    tenant_id: TENANT_ID,
    user_id: ADMIN_USER,
    client_id: ACME_CLIENT,
    title: "Document Review - Bank Statement",
    messages: [
      {
        role: "user",
        content: "Review the bank statement that was just uploaded",
        timestamp: new Date(Date.now() - 15 * 60 * 1000) // 15 mins ago
      },
      {
        role: "assistant",
        content: "I've analyzed the uploaded Bank Statement Q2 (Bank_Statement_Q2.pdf). Here's my review:\n\n**Document Summary:**\n- Period: Q2 2024 (Apr-Jun)\n- Total Transactions: 45\n- Net Inflow: £6,270\n\n**Key Transactions:**\n- £5,000 payment to HMRC (VAT) on 01/04\n- Monthly rent payments: £1,200 each\n- Utilities: £150 average monthly\n\n**Verification:**\n✓ Matches expected document type: Bank Statement\n✓ Period aligns with VAT Return Q2 service requirements\n✓ All transactions legible\n\n**Recommendation:** Approve this document for the Self Assessment Review service.",
        timestamp: new Date(Date.now() - 15 * 60 * 1000 + 5000)
      }
    ],
    context: {
      service_id: "f0000000-0000-0000-0000-000000000004",
      document_ids: ["aa000000-0000-0000-0000-000000000001"],
      action_type: "review"
    },
    created_at: new Date(Date.now() - 15 * 60 * 1000),
    updated_at: new Date(Date.now() - 15 * 60 * 1000)
  },
  {
    tenant_id: TENANT_ID,
    user_id: SARAH_USER,
    client_id: null,
    title: "Client Workload Analysis",
    messages: [
      {
        role: "user",
        content: "How many clients do I have and what's my workload looking like?",
        timestamp: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000) // 3 days ago
      },
      {
        role: "assistant",
        content: "Here's your current workload analysis, Sarah:\n\n**Your Assigned Clients:** 8 clients\n- Acme Corporation Ltd\n- TechStart Ltd\n- Green Energy Solutions\n- City Logistics Ltd\n- Modern Retail Group\n- Classic Motors Ltd\n- Smith & Co Ltd\n- Jones Trading Ltd\n\n**Active Services:** 12 services across your clients\n\n**Priority Items:**\n- 3 overdue services requiring immediate attention\n- 4 documents pending your review\n\n**Workload Assessment:** Your client load is above average for the team. Consider discussing workload redistribution with management if needed.\n\nWould you like me to prioritize your tasks for today?",
        timestamp: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000 + 8000)
      }
    ],
    context: {
      service_id: null,
      document_ids: [],
      action_type: "analysis"
    },
    created_at: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000),
    updated_at: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000)
  }
];

// Insert conversations
const result = db.ai_conversations.insertMany(conversations);
print("Inserted " + result.insertedIds.length + " AI conversations");

// Create indexes for efficient queries
db.ai_conversations.createIndex({ tenant_id: 1, user_id: 1, updated_at: -1 });
db.ai_conversations.createIndex({ tenant_id: 1, client_id: 1 });
print("Created indexes on ai_conversations");

// Verify
const count = db.ai_conversations.countDocuments({ tenant_id: TENANT_ID });
print("\n════════════════════════════════════════════════════════════════");
print("  MONGODB SEED DATA SUMMARY");
print("════════════════════════════════════════════════════════════════");
print("  AI Conversations: " + count);
print("════════════════════════════════════════════════════════════════");
print("  ✓ MongoDB seed data inserted successfully!");
print("════════════════════════════════════════════════════════════════\n");
