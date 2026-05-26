import { Routes, Route, Navigate } from 'react-router-dom'
import MainLayout from './layouts/MainLayout'
import LoginPage from './modules/auth/LoginPage'
import RegisterPage from './modules/auth/RegisterPage'
import DashboardPage from './modules/dashboard/DashboardPage'
import ProductsPage from './modules/products/ProductsPage'
import PostsPage from './modules/posts/PostsPage'
import ContentEditor from './modules/posts/ContentEditor'
import VideoStudioPage from './modules/video-studio/VideoStudioPage'
import SettingsPage from './modules/settings/SettingsPage'

function PrivateRoute({ children }: { children: React.ReactNode }) {
  return localStorage.getItem('token') ? <>{children}</> : <Navigate to="/login" />
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/" element={<PrivateRoute><MainLayout /></PrivateRoute>}>
        <Route index element={<Navigate to="/dashboard" />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="products" element={<ProductsPage />} />
        <Route path="posts" element={<PostsPage />} />
        <Route path="posts/new" element={<ContentEditor />} />
        <Route path="video-studio" element={<VideoStudioPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  )
}
