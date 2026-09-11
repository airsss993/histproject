# План развертывания React + TypeScript + Vite проекта

## 1. Создание проекта

```bash
npm create vite@latest react-ts-app -- --template react-ts
cd react-ts-app
```

## 2. Установка зависимостей

```bash
npm install
npm install sass
npm install -D @types/react @types/react-dom @types/node
```

## 3. Реструктуризация проекта

```
react-ts-app/
├── public/
│   └── index.html
├── src/
│   └── index.tsx
├── vite.config.ts
├── tsconfig.json
├── tsconfig.node.json
├── package.json
└── README.md
```

## 4. Настройка конфигураций

### 4.1 vite.config.ts
```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    open: true,
  }
})
```

### 4.2 tsconfig.json
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

## 5. Создание файлов

### 5.1 public/index.html
```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>React TS App</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/index.tsx"></script>
  </body>
</html>
```

### 5.2 src/index.tsx
```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import './index.scss'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <h1>Хелло реакт ТС</h1>
  </React.StrictMode>,
)
```

### 5.3 src/index.scss
```scss
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  min-height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
  background-color: #f5f5f5;
}

h1 {
  color: #333;
  font-size: 3rem;
}
```

## 6. Настройка package.json

```json
{
  "name": "react-ts-app",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "sass": "^1.69.5"
  },
  "devDependencies": {
    "@types/react": "^18.2.15",
    "@types/react-dom": "^18.2.7",
    "@vitejs/plugin-react": "^4.0.3",
    "typescript": "^5.0.2",
    "vite": "^4.4.5"
  }
}
```

## 7. Запуск проекта

```bash
npm run dev
```

Приложение будет доступно по адресу: http://localhost:3000

## 8. Сборка

```bash
npm run build
npm run preview
```

## 9. Итоговая структура

```
react-ts-app/
├── public/
│   └── index.html
├── src/
│   ├── index.tsx
│   └── index.scss
├── vite.config.ts
├── tsconfig.json
├── tsconfig.node.json
├── package.json
└── README.md
```