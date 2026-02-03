import { useState } from 'react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Textarea } from './ui/textarea';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './ui/select';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from './ui/dialog';
import { useCreateTask } from '../hooks/useTasks';
import { Loader2 } from 'lucide-react';
import type { CreateTaskRequest } from '../lib/api';

interface CreateTaskFormProps {
  trigger?: React.ReactNode;
}

export function CreateTaskForm({ trigger }: CreateTaskFormProps) {
  const [open, setOpen] = useState(false);
  const [formData, setFormData] = useState<CreateTaskRequest>({
    name: '',
    payload: {},
    callback_url: '',
    scheduled_for: undefined,
    timeout_seconds: 30,
    max_retries: 5,
    priority: 0,
    tags: [],
  });

  const createTask = useCreateTask();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    try {
      // Parse and validate payload JSON
      const payload = typeof formData.payload === 'string'
        ? JSON.parse(formData.payload as string)
        : formData.payload;

      // Convert scheduled_for to ISO format with timezone offset
      let scheduled_for = formData.scheduled_for;
      if (scheduled_for) {
        // datetime-local returns "YYYY-MM-DDTHH:mm" format
        // We need to convert it to ISO 8601 with local timezone offset
        const date = new Date(scheduled_for);

        // Get timezone offset in minutes
        const offset = date.getTimezoneOffset();
        const offsetHours = Math.abs(Math.floor(offset / 60));
        const offsetMinutes = Math.abs(offset % 60);
        const offsetSign = offset <= 0 ? '+' : '-';

        // Format: "YYYY-MM-DDTHH:mm:ss+HH:mm"
        const year = date.getFullYear();
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const day = String(date.getDate()).padStart(2, '0');
        const hours = String(date.getHours()).padStart(2, '0');
        const minutes = String(date.getMinutes()).padStart(2, '0');
        const seconds = String(date.getSeconds()).padStart(2, '0');

        scheduled_for = `${year}-${month}-${day}T${hours}:${minutes}:${seconds}${offsetSign}${String(offsetHours).padStart(2, '0')}:${String(offsetMinutes).padStart(2, '0')}`;
      }

      await createTask.mutateAsync({
        ...formData,
        scheduled_for,
        payload,
      } as CreateTaskRequest);

      // Reset form and close dialog on success
      setFormData({
        name: '',
        payload: {},
        callback_url: '',
        scheduled_for: undefined,
        timeout_seconds: 30,
        max_retries: 5,
        priority: 0,
        tags: [],
      });
      setOpen(false);
    } catch (error) {
      alert('Invalid JSON payload. Please check your syntax.');
    }
  };

  const handleInputChange = (field: keyof CreateTaskRequest, value: string | number | string[] | undefined) => {
    setFormData(prev => ({ ...prev, [field]: value }));
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger || (
          <Button>Create Task</Button>
        )}
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create New Task</DialogTitle>
          <DialogDescription>
            Submit a new asynchronous task with callback delivery
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Name */}
          <div className="space-y-2">
            <Label htmlFor="name">Task Name *</Label>
            <Input
              id="name"
              placeholder="e.g., process_order"
              value={formData.name}
              onChange={(e) => handleInputChange('name', e.target.value)}
              required
            />
          </div>

          {/* Callback URL */}
          <div className="space-y-2">
            <Label htmlFor="callback_url">Callback URL *</Label>
            <Input
              id="callback_url"
              type="url"
              placeholder="https://api.example.com/webhooks/task-completed"
              value={formData.callback_url}
              onChange={(e) => handleInputChange('callback_url', e.target.value)}
              required
            />
          </div>

          {/* Payload */}
          <div className="space-y-2">
            <Label htmlFor="payload">Payload (JSON) *</Label>
            <Textarea
              id="payload"
              placeholder='{"order_id": 12345, "customer_id": "abc-123"}'
              className="font-mono text-sm"
              rows={6}
              value={typeof formData.payload === 'string' ? formData.payload : JSON.stringify(formData.payload, null, 2)}
              onChange={(e) => handleInputChange('payload', e.target.value)}
              required
            />
          </div>

          {/* Scheduled For */}
          <div className="space-y-2">
            <Label htmlFor="scheduled_for">Scheduled For (Optional)</Label>
            <Input
              id="scheduled_for"
              type="datetime-local"
              value={formData.scheduled_for || ''}
              onChange={(e) => handleInputChange('scheduled_for', e.target.value || undefined)}
            />
            <p className="text-xs text-muted-foreground">
              Leave empty for immediate execution
            </p>
          </div>

          {/* Priority, Timeout, Max Retries */}
          <div className="grid grid-cols-3 gap-4">
            <div className="space-y-2">
              <Label htmlFor="priority">Priority (0-10)</Label>
              <Select
                value={formData.priority?.toString()}
                onValueChange={(value: string) => handleInputChange('priority', parseInt(value))}
              >
                <SelectTrigger id="priority">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((p) => (
                    <SelectItem key={p} value={p.toString()}>
                      {p}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="timeout">Timeout (seconds)</Label>
              <Input
                id="timeout"
                type="number"
                min="5"
                max="300"
                value={formData.timeout_seconds}
                onChange={(e) => handleInputChange('timeout_seconds', parseInt(e.target.value))}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="retries">Max Retries</Label>
              <Input
                id="retries"
                type="number"
                min="0"
                max="20"
                value={formData.max_retries}
                onChange={(e) => handleInputChange('max_retries', parseInt(e.target.value))}
              />
            </div>
          </div>

          {/* Tags */}
          <div className="space-y-2">
            <Label htmlFor="tags">Tags (comma-separated)</Label>
            <Input
              id="tags"
              placeholder="urgent, email, order-processing"
              value={formData.tags?.join(', ') || ''}
              onChange={(e) => handleInputChange('tags', e.target.value.split(',').map(t => t.trim()).filter(Boolean))}
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={createTask.isPending}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={createTask.isPending}>
              {createTask.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Create Task
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
