'use client';

import { useState, useEffect, useRef } from 'react';
import { PlayIcon, StopIcon, ArrowPathIcon, LinkIcon } from '@heroicons/react/24/outline';

type CrawlResult = {
  type: 'status' | 'error' | 'result';
  message?: string;
  data?: {
    url: string;
    links: string[];
  };
};

export default function Home() {
  const [url, setUrl] = useState('');
  const [depth, setDepth] = useState(2);
  const [workers, setWorkers] = useState(5);
  const [delay, setDelay] = useState(100);
  const [isCrawling, setIsCrawling] = useState(false);
  const [results, setResults] = useState<CrawlResult[]>([]);
  const [activeTab, setActiveTab] = useState<'results' | 'logs'>('results');
  const ws = useRef<WebSocket | null>(null);
  const endOfMessagesRef = useRef<HTMLDivElement>(null);

  // Scroll to bottom of logs when new messages arrive
  useEffect(() => {
    endOfMessagesRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [results]);

  // Initialize WebSocket connection
  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    ws.current = new WebSocket(wsUrl);

    ws.current.onmessage = (event) => {
      const data = JSON.parse(event.data) as CrawlResult;
      setResults(prev => [...prev, data]);
    };

    ws.current.onclose = () => {
      console.log('WebSocket disconnected');
    };

    return () => {
      if (ws.current) {
        ws.current.close();
      }
    };
  }, []);

  const startCrawling = async () => {
    if (!url) return;
    
    setIsCrawling(true);
    setResults([]);

    try {
      const response = await fetch('/api/crawl', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          url,
          depth,
          workers,
          delay,
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to start crawling');
      }
    } catch (error) {
      console.error('Error starting crawl:', error);
      setResults(prev => [...prev, { 
        type: 'error', 
        message: `Error: ${error instanceof Error ? error.message : 'Unknown error'}` 
      }]);
      setIsCrawling(false);
    }
  };

  const stopCrawling = () => {
    // In a real app, you would send a stop signal to the server
    setIsCrawling(false);
    setResults(prev => [...prev, { 
      type: 'status', 
      message: 'Crawl stopped by user' 
    }]);
  };

  const resetForm = () => {
    setUrl('');
    setDepth(2);
    setWorkers(5);
    setDelay(100);
    setResults([]);
  };

  const filteredResults = activeTab === 'results' 
    ? results.filter(r => r.type === 'result')
    : results;

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="max-w-7xl mx-auto px-4 py-8 sm:px-6 lg:px-8">
        <div className="text-center mb-12">
          <h1 className="text-4xl font-bold text-gray-900 mb-2">Go Web Crawler</h1>
          <p className="text-lg text-gray-600">
            A concurrent web crawler built with Go and Next.js
          </p>
        </div>

        <div className="bg-white shadow rounded-lg p-6 mb-8">
          <div className="space-y-6">
            <div>
              <label htmlFor="url" className="block text-sm font-medium text-gray-700">
                Website URL
              </label>
              <div className="mt-1 relative rounded-md shadow-sm">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <LinkIcon className="h-5 w-5 text-gray-400" aria-hidden="true" />
                </div>
                <input
                  type="url"
                  name="url"
                  id="url"
                  className="focus:ring-indigo-500 focus:border-indigo-500 block w-full pl-10 sm:text-sm border-gray-300 rounded-md p-2 border"
                  placeholder="https://example.com"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  disabled={isCrawling}
                />
              </div>
            </div>

            <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
              <div>
                <label htmlFor="depth" className="block text-sm font-medium text-gray-700">
                  Max Depth: {depth}
                </label>
                <input
                  type="range"
                  id="depth"
                  name="depth"
                  min="1"
                  max="10"
                  value={depth}
                  onChange={(e) => setDepth(Number(e.target.value))}
                  className="mt-1 block w-full"
                  disabled={isCrawling}
                />
              </div>

              <div>
                <label htmlFor="workers" className="block text-sm font-medium text-gray-700">
                  Workers: {workers}
                </label>
                <input
                  type="range"
                  id="workers"
                  name="workers"
                  min="1"
                  max="20"
                  value={workers}
                  onChange={(e) => setWorkers(Number(e.target.value))}
                  className="mt-1 block w-full"
                  disabled={isCrawling}
                />
              </div>

              <div>
                <label htmlFor="delay" className="block text-sm font-medium text-gray-700">
                  Delay: {delay}ms
                </label>
                <input
                  type="range"
                  id="delay"
                  name="delay"
                  min="0"
                  max="2000"
                  step="100"
                  value={delay}
                  onChange={(e) => setDelay(Number(e.target.value))}
                  className="mt-1 block w-full"
                  disabled={isCrawling}
                />
              </div>
            </div>

            <div className="flex space-x-4">
              {!isCrawling ? (
                <>
                  <button
                    type="button"
                    onClick={startCrawling}
                    disabled={!url}
                    className={`inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 ${!url ? 'opacity-50 cursor-not-allowed' : ''}`}
                  >
                    <PlayIcon className="-ml-1 mr-2 h-5 w-5" />
                    Start Crawling
                  </button>
                  <button
                    type="button"
                    onClick={resetForm}
                    className="inline-flex items-center px-4 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                  >
                    <ArrowPathIcon className="-ml-1 mr-2 h-5 w-5 text-gray-500" />
                    Reset
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  onClick={stopCrawling}
                  className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
                >
                  <StopIcon className="-ml-1 mr-2 h-5 w-5" />
                  Stop Crawling
                </button>
              )}
            </div>
          </div>
        </div>

        <div className="bg-white shadow rounded-lg overflow-hidden">
          <div className="border-b border-gray-200">
            <nav className="flex -mb-px">
              <button
                onClick={() => setActiveTab('results')}
                className={`py-4 px-6 text-center border-b-2 font-medium text-sm ${activeTab === 'results' ? 'border-indigo-500 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}`}
              >
                Results ({results.filter(r => r.type === 'result').length})
              </button>
              <button
                onClick={() => setActiveTab('logs')}
                className={`py-4 px-6 text-center border-b-2 font-medium text-sm ${activeTab === 'logs' ? 'border-indigo-500 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'}`}
              >
                Logs ({results.length})
              </button>
            </nav>
          </div>

          <div className="p-4 h-96 overflow-y-auto">
            {filteredResults.length === 0 ? (
              <div className="text-center text-gray-500 py-12">
                {activeTab === 'results' 
                  ? 'Crawled results will appear here'
                  : 'Crawling logs will appear here'}
              </div>
            ) : (
              <div className="space-y-4">
                {filteredResults.map((result, index) => (
                  <div 
                    key={index}
                    className={`p-3 rounded-md ${result.type === 'error' ? 'bg-red-50' : result.type === 'status' ? 'bg-blue-50' : 'bg-white border'}`}
                  >
                    {result.type === 'result' && result.data && (
                      <div>
                        <div className="font-medium text-indigo-600">{result.data.url}</div>
                        {result.data.links.length > 0 && (
                          <div className="mt-1 text-sm text-gray-600">
                            Found {result.data.links.length} links
                          </div>
                        )}
                      </div>
                    )}
                    {result.message && (
                      <div className={`text-sm ${result.type === 'error' ? 'text-red-700' : 'text-blue-700'}`}>
                        {result.message}
                      </div>
                    )}
                  </div>
                ))}
                <div ref={endOfMessagesRef} />
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
