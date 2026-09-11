FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .

ARG VITE_API_BASE
ENV VITE_API_BASE=$VITE_API_BASE

RUN npm run build && npm install -g serve

EXPOSE 3000

CMD ["serve", "-s", "dist", "-l", "3000"]
