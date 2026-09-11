// import { watchFetchData } from '../../app/saga/saga'
import { cards } from '../../entities/cons'

export enum EventType {
    Execution = 1,
    MilitaryOperation = 2,
    BorderConflict = 3,
    Uprising = 4
}

export interface EventObject {
    id: number;
    title: string;
    description: string;
    latitude: string;
    longitude: string;
    eventDate: string;
    eventType: EventType;
    previewUrlImage: string;
    siteUrl: string | null;
}

type Coordinates = [number, number];

class EventsDataService {
    private static instance: EventsDataService;
    private _events: EventObject[] = [];
    private _eventsPosition: Map<string, Coordinates> = new Map();
    private _isLoaded = false;
    private _loadingPromise: Promise<void> | null = null;

    private constructor() {}

    static getInstance(): EventsDataService {
        if (!EventsDataService.instance) {
            EventsDataService.instance = new EventsDataService();
        }
        return EventsDataService.instance;
    }

    get events(): EventObject[] {
        return this._events;
    }

    get eventsPosition(): Map<string, Coordinates> {
        return this._eventsPosition;
    }

    get isLoaded(): boolean {
        return this._isLoaded;
    }

    async loadData(): Promise<void> {
        if (this._isLoaded) {
            return;
        }

        if (this._loadingPromise) {
            return this._loadingPromise;
        }

        this._loadingPromise = this.fetchData();
        return this._loadingPromise;
    }

    private async fetchData(): Promise<void> {
        try {
            console.log('Загрузка данных с сервера...');

            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), 10000);

            const results = await Promise.all(
                cards.map(card =>
                    fetch(card, {
                        method: 'GET',
                        signal: controller.signal,
                        mode: 'cors',
                        headers: {
                            'Accept': 'application/json',
                            'Content-Type': 'application/json'
                        }
                    })
                    .then(res => res.ok ? res.json().then((data: any) => {
                        const obj = data.object;
                        return {
                            id: obj.id,
                            title: obj.title,
                            description: obj.description,
                            latitude: obj.latitude,
                            longitude: obj.longitude,
                            eventDate: obj.eventDate,
                            eventType: obj.eventType,
                            previewUrlImage: obj.previewUrlImage,
                            siteUrl: obj.siteUrl ?? null
                        } as EventObject;
                    }) : null)
                    .catch(() => null)
                )
            );

            clearTimeout(timeoutId);
            
            const events = results.filter((e): e is EventObject => e !== null);

            if (events.length === 0) {
                throw new Error('Нет данных в ответе сервера или неверный формат');
            }

            this._events = events;
            this._eventsPosition.clear();

            this._events.forEach((event) => {
                const lat = parseFloat(event.latitude);
                const lng = parseFloat(event.longitude);

                if (!isNaN(lat) && !isNaN(lng)) {
                    this._eventsPosition.set(event.id.toString(), [lat, lng]);
                } else {
                    console.warn(
                        `[EventsDataService] Не удалось распарсить координаты для события ID: ${event.id}, ` +
                        `Title: "${event.title}", Координаты: (${event.latitude}, ${event.longitude}), Сайт: "${event.siteUrl}"`
                    );
                }
            });

            this._isLoaded = true;
            console.log(`✅ Загружено событий: ${this._events.length}`);
        } catch (error) {
            console.error('❌ [EventsDataService] Ошибка при загрузке данных:', error);
            this._isLoaded = false;

            throw error;
        } finally {
            this._loadingPromise = null;
        }
    }

    getEventPosition(eventId: number): Coordinates | undefined {
        return this._eventsPosition.get(eventId.toString());
    }

    getEventById(eventId: number): EventObject | undefined {
        return this._events.find(event => event.id === eventId);
    }
}

export const eventsDataService = EventsDataService.getInstance();
export const EventsPosition = eventsDataService.eventsPosition;