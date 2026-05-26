import { Outlet, NavLink, useNavigate } from 'react-router-dom'

const navItems = [
  { path: '/dashboard', label: 'Dashboard', icon: '📊' },
  { path: '/products', label: 'Products', icon: '🛍️' },
  { path: '/posts', label: 'Posts', icon: '📝' },
  { path: '/video-studio', label: 'Video Studio', icon: '🎬' },
  { path: '/settings', label: 'Settings', icon: '⚙️' },
]

export default function MainLayout() {
  const navigate = useNavigate()

  const handleLogout = () => {
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    navigate('/login')
  }

  return (
    <div className="flex min-h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-64 bg-white border-r border-gray-200 flex flex-col">
        <div className="p-6 border-b border-gray-200">
          <h1 className="text-xl font-bold text-gray-900">TikTok Affiliate</h1>
          <p className="text-xs text-gray-500 mt-1">Content Platform</p>
        </div>

        <nav className="flex-1 p-4 space-y-1">
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                  isActive
                    ? 'bg-purple-50 text-purple-700'
                    : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'
                }`
              }
            >
              <span>{item.icon}</span>
              {item.label}
            </NavLink>
          ))}

          <NavLink
            to="/posts/new"
            className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium bg-purple-600 text-white hover:bg-purple-700 transition-colors mt-4"
          >
            <span>➕</span>
            New Post
          </NavLink>
        </nav>

        <div className="p-4 border-t border-gray-200">
          <button
            onClick={handleLogout}
            className="w-full px-4 py-2 bg-red-50 text-red-600 rounded-lg text-sm font-medium hover:bg-red-100 transition-colors"
          >
            Logout
          </button>
        </div>
      </aside>

      {/* Main content */}
      <div className="flex-1 flex flex-col">
        {/* Topbar */}
        <header className="h-16 bg-white border-b border-gray-200 flex items-center justify-between px-6">
          <div className="text-sm text-gray-500">
            TikTok Affiliate Management Platform
          </div>
          <div className="text-sm text-gray-700 font-medium">
            {(() => {
              try {
                const user = JSON.parse(localStorage.getItem('user') || '{}')
                return user.displayName || user.email || 'User'
              } catch {
                return 'User'
              }
            })()}
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 p-6 overflow-auto">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
