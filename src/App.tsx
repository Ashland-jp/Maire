import { useState } from 'react';
import { ChatInterface } from './components/ChatInterface';
import { ConfigPanel } from './components/ConfigPanel';
import { Header } from './components/Header';
import type { Topology, LLMModel } from './components/ConfigPanel';

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  headerStack?: Array<{
    model: string;
    response: string;
  }>;
}

export default function App() {
  const [topology, setTopology] = useState<Topology>('standard-chain');
  const [models, setModels] = useState<LLMModel[]>([
    { id: 'grok', name: 'Grok', enabled: true },
    { id: 'claude', name: 'Claude', enabled: true },
    { id: 'gpt-4', name: 'GPT-4', enabled: true },
  ]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [isConfigOpen, setIsConfigOpen] = useState(true);

  const handleSendMessage = async (content: string) => {
    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content,
      timestamp: new Date(),
    };

    setMessages((prev) => [...prev, userMessage]);

    // Prepare request for Go backend
    const enabledModels = models.filter((m) => m.enabled).map((m) => m.id);
    
    try {
      const response = await fetch('/maire/run', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          original_prompt: content,
          topology,
          models: enabledModels,
        }),
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data = await response.json();

      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: data.final_response || 'Response from MAIRE',
        timestamp: new Date(),
        headerStack: data.header_stack,
      };

      setMessages((prev) => [...prev, assistantMessage]);
    } catch (error) {
      console.error('Error calling MAIRE backend:', error);
      
      // Show error message to user
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: `Error: Unable to connect to MAIRE backend. Make sure the Go server is running on port 8080.\n\nError details: ${error}`,
        timestamp: new Date(),
      };

      setMessages((prev) => [...prev, errorMessage]);
    }
  };

  return (
    <div className="min-h-screen bg-black flex flex-col">
      <Header 
        isConfigOpen={isConfigOpen}
        onToggleConfig={() => setIsConfigOpen(!isConfigOpen)}
      />
      
      <div className="flex flex-1 overflow-hidden">
        <ConfigPanel
          isOpen={isConfigOpen}
          topology={topology}
          onTopologyChange={setTopology}
          models={models}
          onModelsChange={setModels}
        />
        
        <ChatInterface
          messages={messages}
          onSendMessage={handleSendMessage}
          topology={topology}
          activeModels={models.filter((m) => m.enabled).length}
        />
      </div>
    </div>
  );
}