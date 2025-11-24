import { Send } from 'lucide-react';
import { Button } from './ui/button';
import { Textarea } from './ui/textarea';

interface ChatInputProps {
  value: string;
  onChange: (value: string) => void;
  onSend: () => void;
  disabled?: boolean;
}

export function ChatInput({ value, onChange, onSend, disabled }: ChatInputProps) {
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (!disabled && value.trim()) {
        onSend();
      }
    }
  };

  return (
    <div className="flex gap-3 items-end">
      <div className="flex-1 relative">
        <Textarea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Now make 'em kith"
          disabled={disabled}
          className="min-h-[60px] max-h-[200px] resize-none bg-gray-800/40 border-gray-700/50 text-gray-100 placeholder:text-gray-500 focus:border-gray-600 backdrop-blur-sm"
          rows={2}
        />
      </div>
      
      <Button
        onClick={onSend}
        disabled={disabled || !value.trim()}
        size="lg"
        className="h-[60px] px-6 bg-gray-700/60 hover:bg-gray-700/80 text-gray-100 border border-gray-600/50 backdrop-blur-sm"
      >
        <Send className="size-5" />
      </Button>
    </div>
  );
}
