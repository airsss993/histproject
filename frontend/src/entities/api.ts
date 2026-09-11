import { card, apiServer, checkBack } from './cons'

export interface FetchDataPayload {
  id?: number;
  url?: string;
}

export interface DataResponse {
  data: any;
}

export const api = {// @ts-ignore
  fetchData: async (payload: FetchDataPayload): Promise<DataResponse> => {
    // const url = payload.url ?? (payload.id !== undefined ? card(payload.id) : '');
    const response = await fetch(apiServer);
    if (!response.ok) {
      throw new Error('Network response was not ok');
    }
    return response.json();
  },

  fetchCard: async (id: number): Promise<any> => {
    const response = await fetch(card(id));
    if (!response.ok) {
      throw new Error('Network response was not ok');
    }
    return response.json();
  }
};

(async () => {
  try {
    const res = await fetch(checkBack);
    if (res.status === 200) {
      console.log(`Server: ${checkBack} — status ${res.status} OK`);
    } else {
      console.warn(`Server responded with unexpected status ${res.status}`);
    }
  } catch (e) {
    console.error(`Server unreachable: ${checkBack}`, e);
  }
})();
