import { Check } from 'lucide-react';

interface WorkflowStep {
  label: string;
  status: 'completed' | 'current' | 'upcoming';
}

interface WorkflowStepsProps {
  steps: WorkflowStep[];
}

/**
 * Horizontal step indicator showing workflow progress.
 * Circles connected by lines with status-based coloring:
 * - Completed: success-solid fill with check icon
 * - Current: red-500 fill (brand accent)
 * - Upcoming: n-200 fill (neutral)
 *
 * @validates Requirements 6.1
 */
export function WorkflowSteps({ steps }: WorkflowStepsProps) {
  return (
    <div className="flex items-center gap-0" role="list" aria-label="Workflow steps">
      {steps.map((step, index) => (
        <div key={step.label} className="flex items-center" role="listitem">
          {/* Step circle */}
          <div className="flex flex-col items-center">
            <div
              className={`flex items-center justify-center h-8 w-8 rounded-full text-sm font-medium ${getCircleStyle(step.status)}`}
              aria-current={step.status === 'current' ? 'step' : undefined}
            >
              {step.status === 'completed' ? (
                <Check className="h-4 w-4" aria-hidden="true" />
              ) : (
                <span>{index + 1}</span>
              )}
            </div>
            <span
              className={`mt-1 text-xs font-medium ${getLabelStyle(step.status)}`}
            >
              {step.label}
            </span>
          </div>

          {/* Connecting line (not after last step) */}
          {index < steps.length - 1 && (
            <div
              className={`h-0.5 w-8 mx-1 ${getLineStyle(step.status)}`}
              aria-hidden="true"
            />
          )}
        </div>
      ))}
    </div>
  );
}

function getCircleStyle(status: WorkflowStep['status']): string {
  switch (status) {
    case 'completed':
      return 'bg-success-solid text-white';
    case 'current':
      return 'bg-red-500 text-white';
    case 'upcoming':
      return 'bg-n-200 text-n-500';
  }
}

function getLabelStyle(status: WorkflowStep['status']): string {
  switch (status) {
    case 'completed':
      return 'text-success-fg';
    case 'current':
      return 'text-red-600';
    case 'upcoming':
      return 'text-n-400';
  }
}

function getLineStyle(status: WorkflowStep['status']): string {
  switch (status) {
    case 'completed':
      return 'bg-success-solid';
    case 'current':
      return 'bg-red-500';
    case 'upcoming':
      return 'bg-n-200';
  }
}
