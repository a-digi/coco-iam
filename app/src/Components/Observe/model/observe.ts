export interface ProcessMetrics {
  name: string;
  pid?: number;
  found: boolean;
  cpu_pct: number;
  rss_bytes: number;
  threads: number;
  error?: string;
}

export interface ProcessTarget {
  name: string;
}

export interface DiskMetrics {
  mount: string;
  used_bytes: number;
  available_bytes: number;
  total_bytes: number;
}

export interface NetMetrics {
  iface: string;
  rx_bytes: number;
  tx_bytes: number;
  rx_errors: number;
  tx_errors: number;
}

export interface OSMetrics {
  mem_total_bytes: number;
  mem_used_bytes: number;
  mem_available_bytes: number;
  swap_total_bytes: number;
  swap_used_bytes: number;
  cpu_load_1m: number;
  cpu_load_5m: number;
  cpu_load_15m: number;
  cpu_usage_pct: number;
  disks: DiskMetrics[];
  network: NetMetrics[];
}

export interface BatchPayload {
  agent_id: string;
  timestamp: string;
  sequence: number;
  buffered: boolean;
  os: OSMetrics | null;
  processes: ProcessMetrics[];
}

export interface Batch {
  id: number;
  agent_id: string;
  captured_at: string;
  received_at: string;
  sequence: number;
  buffered: boolean;
  payload: BatchPayload;
}

export interface Agent {
  id: string;
  name: string;
  api_key: string;
  registered_at: string;
  last_seen_at?: string;
  enabled: boolean;
  has_binary: boolean;
  track_os: boolean;
  processes: ProcessTarget[];
}

export interface CreateAgentRequest {
  name: string;
  processes: ProcessTarget[];
  track_os: boolean;
}

export interface CreateAgentResponse {
  id: string;
  name: string;
  api_key: string;
  api_secret: string;
  processes: ProcessTarget[];
  track_os: boolean;
}
