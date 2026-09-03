'use client';

import { useState, useRef, useEffect } from 'react';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  actions?: ActionButton[];
}

interface ActionButton {
  label: string;
  action: string;
  variant?: 'primary' | 'secondary' | 'danger';
}

interface UrgentItem {
  id: string;
  type: 'deadline' | 'document' | 'email';
  title: string;
  subtitle: string;
  action: string;
  actionLabel: string;
  urgency: 'overdue' | 'today' | 'soon';
}

interface AIChatProps {
  userName?: string;
  urgentItems?: UrgentItem[];
  onAction?: (action: string, itemId?: string) => void;
  minimized?: boolean;
}

export function AIChat({ userName = 'there', urgentItems = [], onAction, minimized = false }: AIChatProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isTyping, setIsTyping] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Sample urgent items for demo
  const sampleUrgentItems: UrgentItem[] = urgentItems.length > 0 ? urgentItems : [
    {
      id: '1',
      type: 'deadline',
      title: 'ABC Corp VAT due tomorrow',
      subtitle: 'Q2 2024 VAT Return',
      action: 'file',
      actionLabel: 'File →',
      urgency: 'today',
    },
    {
      id: '2',
      type: 'document',
      title: 'Review Bank Statement',
      subtitle: 'DEF Inc · Uploaded 10 min ago',
      action: 'review',
      actionLabel: 'Review →',
      urgency: 'today',
    },
    {
      id: '3',
      type: 'email',
      title: '2 emails need reply',
      subtitle: 'From clients awaiting response',
      action: 'reply',
      actionLabel: 'Reply →',
      urgency: 'soon',
    },
  ];

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim()) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input,
      timestamp: new Date(),
    };

    setMessages(prev => [...prev, userMessage]);
    setInput('');
    setIsTyping(true);

    // Simulate AI response (replace with actual API call)
    setTimeout(() => {
      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: getAIResponse(input),
        timestamp: new Date(),
        actions: getActionsForQuery(input),
      };
      setMessages(prev => [...prev, assistantMessage]);
      setIsTyping(false);
    }, 1000);
  };

  const getAIResponse = (query: string): string => {
    const lowerQuery = query.toLowerCase();
    if (lowerQuery.includes('overdue') || lowerQuery.includes('chase')) {
      return 'You have 3 overdue items. Would you like me to send chase emails to all of them?';
    }
    if (lowerQuery.includes('vat') || lowerQuery.includes('deadline')) {
      return 'ABC Corp VAT is due tomorrow. XYZ Ltd VAT is due in 5 days. Would you like to see all upcoming deadlines?';
    }
    if (lowerQuery.includes('client') && lowerQuery.includes('add')) {
      return 'I can help you add a new client. Would you like to search Companies House or enter details manually?';
    }
    return "I'm here to help! You can ask me about clients, deadlines, documents, or say things like 'chase all overdue' or 'show troublemakers'.";
  };

  const getActionsForQuery = (query: string): ActionButton[] | undefined => {
    const lowerQuery = query.toLowerCase();
    if (lowerQuery.includes('overdue') || lowerQuery.includes('chase')) {
      return [
        { label: 'Chase All', action: 'chase-all', variant: 'primary' },
        { label: 'View List', action: 'view-overdue', variant: 'secondary' },
      ];
    }
    if (lowerQuery.includes('client') && lowerQuery.includes('add')) {
      return [
        { label: 'Search CH', action: 'search-ch', variant: 'primary' },
        { label: 'Manual Entry', action: 'manual-add', variant: 'secondary' },
      ];
    }
    return undefined;
  };

  const handleAction = (action: string, itemId?: string) => {
    onAction?.(action, itemId);
  };

  const getUrgencyColor = (urgency: string) => {
    switch (urgency) {
      case 'overdue': return 'border-red-500 bg-red-50 dark:bg-red-900/20';
      case 'today': return 'border-orange-500 bg-orange-50 dark:bg-orange-900/20';
      case 'soon': return 'border-yellow-500 bg-yellow-50 dark:bg-yellow-900/20';
      default: return 'border-gray-300 bg-gray-50 dark:bg-gray-800';
    }
  };

  const getUrgencyIcon = (urgency: string) => {
    switch (urgency) {
      case 'overdue': return '🔴';
      case 'today': return '🟠';
      case 'soon': return '🟡';
      default: return '🟢';
    }
  };

  if (minimized) {
    return (
      <div className="bg-white dark:bg-slate-800 rounded-lg shadow-sm p-4 border border-gray-200 dark:border-gray-700">
        <div className="flex items-center gap-2 text-gray-600 dark:text-gray-400">
          <span className="text-xl">🤖</span>
          <span className="text-sm">AI Chat available - click to expand</span>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-white dark:bg-slate-800 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700">
      {/* Header */}
      <div className="p-4 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center gap-2">
          <span className="text-2xl">🤖</span>
          <div>
            <h2 className="font-semibold text-gray-900 dark:text-white">
              Good {getGreeting()}, {userName}!
            </h2>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              AI Assistant · Always here to help
            </p>
          </div>
        </div>
      </div>

      {/* Urgent Items (shown when no messages) */}
      {messages.length === 0 && sampleUrgentItems.length > 0 && (
        <div className="p-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-2 mb-3">
            <span className="text-red-500 font-semibold text-sm">🔴 {sampleUrgentItems.length} URGENT TODAY</span>
          </div>
          <div className="space-y-2">
            {sampleUrgentItems.map((item) => (
              <div
                key={item.id}
                className={`flex items-center justify-between p-3 rounded-lg border-l-4 ${getUrgencyColor(item.urgency)}`}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span>{getUrgencyIcon(item.urgency)}</span>
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                      {item.title}
                    </p>
                    <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                      {item.subtitle}
                    </p>
                  </div>
                </div>
                <button
                  onClick={() => handleAction(item.action, item.id)}
                  className="ml-2 px-3 py-1 text-xs font-medium text-white bg-blue-600 rounded hover:bg-blue-700 whitespace-nowrap"
                >
                  {item.actionLabel}
                </button>
              </div>
            ))}
          </div>

          {/* Quick Actions */}
          <div className="flex gap-2 mt-4">
            <button
              onClick={() => handleAction('chase-all')}
              className="flex-1 px-3 py-2 text-sm font-medium text-blue-600 bg-blue-50 dark:bg-blue-900/30 dark:text-blue-400 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/50"
            >
              Chase All Overdue
            </button>
            <button
              onClick={() => handleAction('whats-next')}
              className="flex-1 px-3 py-2 text-sm font-medium text-gray-600 bg-gray-100 dark:bg-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
            >
              What's next?
            </button>
          </div>
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.map((message) => (
          <div
            key={message.id}
            className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
          >
            <div
              className={`max-w-[80%] rounded-lg p-3 ${
                message.role === 'user'
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white'
              }`}
            >
              <p className="text-sm">{message.content}</p>
              {message.actions && (
                <div className="flex gap-2 mt-2">
                  {message.actions.map((action, idx) => (
                    <button
                      key={idx}
                      onClick={() => handleAction(action.action)}
                      className={`px-3 py-1 text-xs font-medium rounded ${
                        action.variant === 'primary'
                          ? 'bg-blue-500 text-white hover:bg-blue-600'
                          : 'bg-white dark:bg-slate-600 text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-slate-500'
                      }`}
                    >
                      {action.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        ))}

        {isTyping && (
          <div className="flex justify-start">
            <div className="bg-gray-100 dark:bg-slate-700 rounded-lg p-3">
              <div className="flex gap-1">
                <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0ms' }}></span>
                <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '150ms' }}></span>
                <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '300ms' }}></span>
              </div>
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <form onSubmit={handleSubmit} className="p-4 border-t border-gray-200 dark:border-gray-700">
        <div className="flex gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask anything... (e.g., 'chase all overdue')"
            className="flex-1 px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-slate-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
          <button
            type="submit"
            disabled={!input.trim()}
            className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Send
          </button>
        </div>
        <div className="flex gap-2 mt-2">
          <button
            type="button"
            onClick={() => setInput('Show troublemakers')}
            className="px-2 py-1 text-xs text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-slate-700 rounded hover:bg-gray-200 dark:hover:bg-slate-600"
          >
            Show troublemakers
          </button>
          <button
            type="button"
            onClick={() => setInput('What needs attention?')}
            className="px-2 py-1 text-xs text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-slate-700 rounded hover:bg-gray-200 dark:hover:bg-slate-600"
          >
            What needs attention?
          </button>
          <button
            type="button"
            onClick={() => setInput('Add new client')}
            className="px-2 py-1 text-xs text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-slate-700 rounded hover:bg-gray-200 dark:hover:bg-slate-600"
          >
            Add client
          </button>
        </div>
      </form>
    </div>
  );
}

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return 'morning';
  if (hour < 17) return 'afternoon';
  return 'evening';
}

export default AIChat;
