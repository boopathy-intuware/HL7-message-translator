import { Route, Routes } from 'react-router-dom'
import './App.css'
import { Layout } from './components/Layout'
import { ConverterPage } from './pages/ConverterPage'
import { FhirExplorerPage } from './pages/FhirExplorerPage'
import { InboxPage } from './pages/InboxPage'
import { MessageDetailPage } from './pages/MessageDetailPage'

function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<ConverterPage />} />
        <Route path="/inbox" element={<InboxPage />} />
        <Route path="/fhir" element={<FhirExplorerPage />} />
        <Route path="/messages/:id" element={<MessageDetailPage />} />
      </Route>
    </Routes>
  )
}

export default App