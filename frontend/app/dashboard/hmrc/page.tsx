'use client';

import { useState } from 'react';
import { HMRCOverviewPanel } from './components/HMRCOverviewPanel';
import { VATPanel } from './components/VATPanel';
import { CT600Panel } from './components/CT600Panel';
import { SelfAssessmentPanel } from './components/SelfAssessmentPanel';
import { HMRCDeadlinesPanel } from './components/HMRCDeadlinesPanel';

export default function HMRCPage() {
  const [activeTab, setActiveTab] = useState('overview');

  const tabs = [
    { id: 'overview', label: 'Overview', icon: '🏛️' },
    { id: 'vat', label: 'VAT Returns', icon: '📊' },
    { id: 'ct600', label: 'CT600', icon: '🏢' },
    { id: 'sa', label: 'Self Assessment', icon: '👤' },
    { id: 'deadlines', label: 'Deadlines', icon: '📅' },
  ];

  const renderPanel = () => {
    switch (activeTab) {
      case 'overview':
        return <HMRCOverviewPanel />;
      case 'vat':
        return <VATPanel />;
      case 'ct600':
        return <CT600Panel />;
      case 'sa':
        return <SelfAssessmentPanel />;
      case 'deadlines':
        return <HMRCDeadlinesPanel />;
      default:
        return <HMRCOverviewPanel />;
    }
  };

  return (
    <div className="h-full flex flex-col bg-gray-50 dark:bg-slate-900">
      {/* Header with Tabs */}
      <div className="bg-white dark:bg-slate-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h1 className="text-xl font-semibold text-gray-900 dark:text-white flex items-center gap-2">
              <span>🏛️</span> HMRC Filing
            </h1>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Manage VAT returns, Corporation Tax, and Self Assessment
            </p>
          </div>
          <button className="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700">
            + New Filing
          </button>
        </div>

        {/* Tab Navigation */}
        <div className="flex gap-1 border-b border-gray-200 dark:border-gray-700 -mb-4">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`
                flex items-center gap-2 px-4 py-3 text-sm font-medium transition-colors
                ${activeTab === tab.id
                  ? 'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400'
                  : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
                }
              `}
            >
              <span>{tab.icon}</span>
              <span>{tab.label}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Panel Content */}
      <div className="flex-1 overflow-hidden">
        {renderPanel()}
      </div>
    </div>
  );
}
