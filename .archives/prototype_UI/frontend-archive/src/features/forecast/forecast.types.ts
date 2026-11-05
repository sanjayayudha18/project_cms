export interface ForecastRecord {
  id: string;
  atmId: string;
  currentBalance: number;
  predictedUsageH1: number;
  predictedUsageH2: number;
  recommendedReplenishment: number;
  priority: 'High' | 'Medium' | 'Low';
}

export interface EnrichedForecastRecord extends ForecastRecord {
  location: string;
  vendorName: string;
}

export interface ScheduleEntry {
  id: string;
  atmId: string;
  location: string;
  vendorName: string;
  scheduledDate: string;
  amount: number;
}
