class CrawlerUI {
    constructor() {
        this.resultsDiv = document.getElementById('results');
        this.statsSpan = document.getElementById('stats');
        this.startButton = document.getElementById('startCrawl');
        this.stopButton = document.getElementById('stopCrawl');
        this.urlInput = document.getElementById('url');
        this.depthInput = document.getElementById('depth');
        this.workersInput = document.getElementById('workers');
        this.delayInput = document.getElementById('delay');
        this.progressBar = document.getElementById('progressBar');
        this.progressText = document.getElementById('progressText');
        this.summaryDiv = document.getElementById('summary');
        this.ws = null;
        this.connected = false;
        this.crawledCount = 0;
        this.totalPages = 0;
        this.startTime = null;
        this.crawlActive = false;
        this.crawlId = null;
        this.isCrawling = false;

        this.initializeWebSocket();
        this.setupEventListeners();
        this.updateUIState();
    }

    initializeWebSocket() {
        try {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws`;
            console.log('Connecting to WebSocket:', wsUrl);

            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                console.log('WebSocket connected successfully');
                this.connected = true;
                this.updateUIState();
            };

            this.ws.onmessage = (event) => {
                console.log('Received message:', event.data);
                try {
                    const response = JSON.parse(event.data);
                    this.handleMessage(response);
                } catch (e) {
                    console.error('Error parsing WebSocket message:', e, 'Raw data:', event.data);
                    this.addLogMessage('error', 'Error processing server response');
                }
            };

            this.ws.onclose = (event) => {
                console.log('WebSocket disconnected:', event.code, event.reason);
                this.connected = false;
                this.updateUIState();
                this.addLogMessage('warning', `Disconnected from server (${event.code}${event.reason ? ': ' + event.reason : ''}). Reconnecting...`);
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                this.addLogMessage('error', 'WebSocket connection error');
            };
        } catch (e) {
            console.error('Error setting up WebSocket:', e);
            this.addLogMessage('error', 'Failed to connect to server. Please refresh the page to try again.');
        }
    }

    setupEventListeners() {
        this.startButton.addEventListener('click', () => this.startCrawl());
        this.stopButton.addEventListener('click', () => this.stopCrawl());
    }

    updateUIState() {
        if (this.connected) {
            this.startButton.disabled = false;
        } else {
            this.startButton.disabled = true;
        }

        if (this.crawlActive) {
            this.stopButton.style.display = 'inline-block';
        } else {
            this.stopButton.style.display = 'none';
        }
    }

    handleMessage(message) {
        console.log('Received message:', message);

        try {
            switch (message.type) {
                case 'connected':
                    this.handleConnected(message);
                    break;
                case 'start':
                    this.handleCrawlStart(message);
                    break;
                case 'result':
                    this.handleCrawlResult(message);
                    break;
                case 'progress':
                    this.updateProgress(message.data.crawled || 0, message.data.total || 0);
                    break;
                case 'complete':
                    this.handleCrawlComplete();
                    break;
                case 'error':
                    this.handleError(message);
                    this.finishCrawl('error');
                    break;
                case 'stopped':
                    this.addLogMessage('info', 'Crawl stopped successfully');
                    this.finishCrawl('stopped');
                    break;
                default:
                    console.log('Unknown message type:', message.type);
            }
        } catch (error) {
            console.error('Error processing message:', error, message);
            this.addLogMessage('error', `Error: ${error.message}`);
        }
    }

    handleCrawlStart(message) {
        this.crawlId = message.crawlId;
    }

    startCrawl() {
        if (!this.connected) {
            alert('Not connected to the server. Please try again.');
            return;
        }

        const url = this.urlInput.value.trim();
        const depth = parseInt(this.depthInput.value, 10);
        const workers = parseInt(this.workersInput.value, 10);
        const delay = parseInt(this.delayInput.value, 10);

        if (!url) {
            alert('Please enter a valid URL');
            return;
        }

        // Clear previous results and reset UI
        this.resultsDiv.innerHTML = '';
        this.resetUI();
        this.crawlActive = true;
        this.startTime = new Date();
        this.updateUIState();

        // Show loading state
        this.startButton.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Crawling...';
        this.stopButton.style.display = 'inline-block';

        // Send start crawl message
        this.ws.send(JSON.stringify({
            type: 'start',
            url: url,
            depth: depth,
            workers: workers,
            delay: delay
        }));

        // Log the start
        this.addLogMessage('info', `Starting crawl of ${url} (depth: ${depth}, workers: ${workers})`);
    }

    stopCrawl() {
        if (!this.connected || !this.crawlActive) return;

        this.ws.send(JSON.stringify({
            type: 'stop'
        }));

        this.addLogMessage('warning', 'Crawl stopped by user');
        this.finishCrawl('stopped');
    }

    finishCrawl(status = 'completed') {
        this.crawlActive = false;
        this.startButton.disabled = false;
        this.startButton.innerHTML = 'Start Crawl';
        this.stopButton.style.display = 'none';

        if (status === 'completed') {
            this.addLogMessage('success', 'Crawl completed successfully!');
        }

        this.showSummary();
        this.updateUIState();
    }

    addLogMessage(type, message) {
        const now = new Date();
        const timeStr = now.toLocaleTimeString();
        const logElement = document.createElement('div');
        logElement.className = `log-message log-${type}`;
        logElement.innerHTML = `<span class="font-mono text-xs opacity-75">[${timeStr}]</span> ${message}`;
        this.resultsDiv.prepend(logElement);
    }

    updateProgress(current, total) {
        this.totalPages = Math.max(this.totalPages, total);
        const progress = this.totalPages > 0 ? Math.min(100, Math.round((current / this.totalPages) * 100)) : 0;
        this.progressBar.style.width = `${progress}%`;
        this.progressText.textContent = `${progress}%`;
        this.statsSpan.textContent = `${current} of ${this.totalPages} pages crawled`;
    }

    showSummary() {
        if (!this.startTime) return;
        
        const endTime = new Date();
        const duration = (endTime - this.startTime) / 1000; // in seconds
        const pagesPerSecond = duration > 0 ? (this.crawledCount / duration).toFixed(2) : this.crawledCount;

        this.summaryDiv.innerHTML = `
            <div class="bg-gray-50 p-4 rounded-lg border border-gray-200">
                <h3 class="text-lg font-semibold mb-3 text-gray-800">Crawl Summary</h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                    <div class="flex justify-between items-center">
                        <span class="text-gray-600 font-medium">Status:</span>
                        <span class="font-mono text-gray-800">Completed</span>
                    </div>
                    <div class="flex justify-between items-center">
                        <span class="text-gray-600 font-medium">Duration:</span>
                        <span class="font-mono text-gray-800">${duration.toFixed(2)} seconds</span>
                    </div>
                    <div class="flex justify-between items-center">
                        <span class="text-gray-600 font-medium">Pages Crawled:</span>
                        <span class="font-mono text-gray-800">${this.crawledCount}</span>
                    </div>
                    <div class="flex justify-between items-center">
                        <span class="text-gray-600 font-medium">Speed:</span>
                        <span class="font-mono text-gray-800">${pagesPerSecond} pages/second</span>
                    </div>
                </div>
            </div>
        `;
    }

    handleError(message) {
        this.addLogMessage('error', `Error: ${message.message || message}`);
    }
    handleCrawlResult(message) {
        this.crawledCount++;
        this.updateProgress(this.crawledCount, message.data.total || this.crawledCount);

        const result = message.data;
        const resultElement = document.createElement('div');
        resultElement.className = 'mb-3 bg-white rounded-lg shadow-sm overflow-hidden border border-gray-200';

        // Create header with URL and toggle button
        const headerElement = document.createElement('div');
        headerElement.className = 'flex justify-between items-center p-3 bg-gray-50 hover:bg-gray-100 cursor-pointer';

        const urlElement = document.createElement('div');
        urlElement.className = 'font-mono text-sm text-blue-600 truncate flex-1';
        urlElement.textContent = result.url;

        const toggleButton = document.createElement('button');
        toggleButton.className = 'text-gray-500 hover:text-gray-700 ml-2';
        toggleButton.innerHTML = `
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
            </svg>
        `;

        headerElement.appendChild(urlElement);
        headerElement.appendChild(toggleButton);

        // Create content area (initially hidden)
        const contentElement = document.createElement('div');
        contentElement.className = 'hidden p-3 border-t border-gray-100';

        // Add status
        if (result.status) {
            const statusElement = document.createElement('div');
            statusElement.className = 'text-sm mb-2';
            statusElement.innerHTML = `<span class="font-medium">Status:</span> ${result.status}`;
            contentElement.appendChild(statusElement);
        }

        // Add links if available
        if (result.links && result.links.length > 0) {
            const linksHeader = document.createElement('div');
            linksHeader.className = 'font-medium text-sm mb-1';
            linksHeader.textContent = `Found ${result.links.length} links:`;
            contentElement.appendChild(linksHeader);

            const linksList = document.createElement('div');
            linksList.className = 'ml-4 max-h-40 overflow-y-auto text-xs font-mono bg-gray-50 p-2 rounded';

            result.links.forEach(link => {
                const linkElement = document.createElement('div');
                linkElement.className = 'py-1 border-b border-gray-100 last:border-0';

                const linkAnchor = document.createElement('a');
                linkAnchor.href = link;
                linkAnchor.target = '_blank';
                linkAnchor.rel = 'noopener noreferrer';
                linkAnchor.className = 'text-blue-600 hover:underline';
                linkAnchor.textContent = link;

                linkElement.appendChild(linkAnchor);
                linksList.appendChild(linkElement);
            });

            contentElement.appendChild(linksList);
        }

        // Add error if present
        if (result.error) {
            const errorElement = document.createElement('div');
            errorElement.className = 'mt-2 p-2 bg-red-50 text-red-700 text-sm rounded';
            errorElement.innerHTML = `<span class="font-medium">Error:</span> ${result.error}`;
            contentElement.appendChild(errorElement);
        }

        // Toggle content visibility when header is clicked
        headerElement.addEventListener('click', () => {
            const isExpanded = contentElement.classList.toggle('hidden');
            toggleButton.innerHTML = `
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                          d="${isExpanded ? 'M19 9l-7 7-7-7' : 'M5 15l7-7 7 7'}">
                    </path>
                </svg>
            `;
        });

        resultElement.appendChild(headerElement);
        resultElement.appendChild(contentElement);
        this.resultsDiv.prepend(resultElement);
    }

    handleError(message) {
        this.addLogMessage('error', `Error: ${message.message}`);
    }

    handleCrawlComplete() {
        this.addLogMessage('success', 'Crawl completed successfully!');
        this.showSummary();
        this.finishCrawl('completed');
    }

    resetUI() {
        this.isCrawling = false;
        this.crawlActive = false;
        this.startButton.disabled = false;
        this.stopButton.disabled = true;
        this.startButton.innerHTML = 'Start Crawl';
        this.stopButton.style.display = 'none';
        this.crawledCount = 0;
        this.totalPages = 0;
        this.startTime = null;
        this.progressBar.style.width = '0%';
        this.progressText.textContent = '0%';
        this.summaryDiv.innerHTML = '';
    }
    
    sendMessage(message) {
        if (!this.ws) {
            this.logMessage('Not connected to server. Please refresh the page.', 'error');
            return false;
        }
        
        if (this.ws.readyState !== WebSocket.OPEN) {
            this.logMessage('Connection not ready. State: ' + this.ws.readyState, 'error');
            return false;
        }
        
        try {
            const messageStr = JSON.stringify(message);
            console.log('Sending message:', messageStr);
            this.ws.send(messageStr);
            return true;
        } catch (e) {
            console.error('Error sending message:', e);
            this.logMessage('Error sending request: ' + e.message, 'error');
            return false;
        }
    }
    
    logMessage(message, type = 'info') {
        const messageElement = document.createElement('div');
        messageElement.className = `p-2 mb-2 rounded ${
            type === 'error' ? 'bg-red-100 text-red-800' :
            type === 'success' ? 'bg-green-100 text-green-800' :
            type === 'warning' ? 'bg-yellow-100 text-yellow-800' :
            'bg-blue-100 text-blue-800'
        }`;
        
        const timestamp = new Date().toISOString().substr(11, 8);
        messageElement.textContent = `[${timestamp}] ${message}`;
        this.resultsDiv.prepend(messageElement);
    }
}

// Initialize the UI when the DOM is fully loaded
document.addEventListener('DOMContentLoaded', () => {
    window.crawlerUI = new CrawlerUI();
});
