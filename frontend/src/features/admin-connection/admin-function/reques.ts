import { adminList } from '../../../entities/cons'

export interface Request {
  adminComment: string
  id: string
  title: string
  date: string
  status: string
  description: string
  eventDate: string
  eventTypeId: string
  telegram: string
  email: string
  siteUrl: string
  comment?: string
}

export interface FetchRequestsParams {
  status?: string
  q?: string
  page?: number
  limit?: number
}

export interface FetchRequestsResult {
  requests: Request[]
  total: number
}

export const fetchRequests = async (params: {
  status?: string;
  q?: string;
  limit?: number;
}) => {
  try {
    const queryParams = new URLSearchParams();
    if (params.status) queryParams.append('status', params.status);
    if (params.q) queryParams.append('q', params.q);
    if (params.limit) queryParams.append('limit', String(params.limit));
    
    const response = await fetch(`${adminList}?${queryParams}`, {
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
      }
    });
    
    if (!response.ok) {
      if (response.status === 401) {
        // Токен истек, нужно обновить
        throw new Error('Unauthorized');
      }
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    
    const data = await response.json();
    return { requests: data.requests || [], total: data.total || 0 };
  } catch (error) {
    console.error('fetchRequests error:', error);
    throw error;
  }
};
