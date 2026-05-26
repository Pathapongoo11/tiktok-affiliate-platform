import { useEffect, useState } from 'react'
import client from '../../api/client'
import type { Product } from '../../types'

const CATEGORIES = ['Beauty', 'Health', 'Home', 'Fashion', 'Food', 'Pet', 'Other']

function commissionBadge(rate: number) {
  if (rate > 40) return 'bg-red-100 text-red-700'
  if (rate >= 10 && rate <= 20) return 'bg-green-100 text-green-700'
  return 'bg-yellow-100 text-yellow-700'
}

function scoreBadge(score: number) {
  if (score >= 80) return 'bg-green-50 text-green-600'
  if (score >= 60) return 'bg-yellow-50 text-yellow-600'
  return 'bg-red-50 text-red-600'
}

interface FormState {
  name: string
  price: string
  commissionRate: string
  category: string
  description: string
}

export default function ProductsPage() {
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [modalOpen, setModalOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form, setForm] = useState<FormState>({
    name: '', price: '', commissionRate: '', category: 'Beauty', description: '',
  })
  const [formError, setFormError] = useState('')

  const fetchProducts = () => {
    setLoading(true)
    client.get('/products')
      .then((res) => setProducts(Array.isArray(res.data) ? res.data : []))
      .catch(() => setProducts([]))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchProducts() }, [])

  const handleAdd = async () => {
    setFormError('')
    if (!form.name || !form.price || !form.commissionRate) {
      setFormError('Name, price, and commission rate are required.')
      return
    }
    setSaving(true)
    try {
      await client.post('/products', {
        name: form.name,
        price: parseFloat(form.price),
        commissionRate: parseFloat(form.commissionRate),
        category: form.category,
        description: form.description,
      })
      setModalOpen(false)
      setForm({ name: '', price: '', commissionRate: '', category: 'Beauty', description: '' })
      fetchProducts()
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } }
      setFormError(e.response?.data?.error || 'Failed to add product.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-gray-900">Products</h2>
          <p className="text-gray-500 text-sm mt-1">Manage your affiliate products</p>
        </div>
        <button
          onClick={() => setModalOpen(true)}
          className="px-4 py-2 bg-purple-600 text-white rounded-lg text-sm font-medium hover:bg-purple-700 transition-colors"
        >
          + Add Product
        </button>
      </div>

      {loading ? (
        <div className="text-center py-12 text-gray-400">Loading products...</div>
      ) : products.length === 0 ? (
        <div className="text-center py-12 text-gray-400">
          No products yet. Add your first affiliate product!
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
          {products.map((product) => (
            <div key={product.id} className="bg-white rounded-xl border border-gray-200 shadow-sm hover:shadow-md transition-shadow p-5 space-y-3">
              <div className="flex items-start justify-between">
                <h3 className="font-semibold text-gray-900 leading-tight">{product.name}</h3>
                <span className={`text-xs font-bold px-2 py-1 rounded-full ${scoreBadge(product.score)}`}>
                  {product.score}
                </span>
              </div>

              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-lg font-bold text-gray-800">
                  ฿{product.price.toLocaleString()}
                </span>
                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${commissionBadge(product.commissionRate)}`}>
                  {product.commissionRate}% commission
                </span>
              </div>

              <div className="flex items-center gap-2">
                <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600">
                  {product.category}
                </span>
                {!product.isActive && (
                  <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-700">
                    Inactive
                  </span>
                )}
              </div>

              {product.flags?.warnings && product.flags.warnings.length > 0 && (
                <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-2">
                  {product.flags.warnings.map((w, i) => (
                    <p key={i} className="text-xs text-yellow-700">⚠️ {w}</p>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Add Product Modal */}
      {modalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div
            className="absolute inset-0 bg-black/50 backdrop-blur-sm"
            onClick={() => setModalOpen(false)}
          />
          <div className="relative bg-white rounded-2xl shadow-2xl w-full max-w-md mx-4 p-6 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold text-gray-900">Add New Product</h3>
              <button
                onClick={() => setModalOpen(false)}
                className="text-gray-400 hover:text-gray-600 text-xl leading-none"
              >
                ×
              </button>
            </div>

            {formError && (
              <div className="bg-red-50 border border-red-200 text-red-700 px-3 py-2 rounded text-sm">
                {formError}
              </div>
            )}

            <div className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Product Name *</label>
                <input
                  type="text"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Price (THB) *</label>
                <input
                  type="number"
                  value={form.price}
                  onChange={(e) => setForm({ ...form, price: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Commission Rate (%) *</label>
                <input
                  type="number"
                  value={form.commissionRate}
                  onChange={(e) => setForm({ ...form, commissionRate: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Category</label>
                <select
                  value={form.category}
                  onChange={(e) => setForm({ ...form, category: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
                >
                  {CATEGORIES.map((cat) => (
                    <option key={cat} value={cat}>{cat}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                <input
                  type="text"
                  value={form.description}
                  onChange={(e) => setForm({ ...form, description: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
                />
              </div>
            </div>

            <div className="flex gap-3 pt-2">
              <button
                onClick={() => setModalOpen(false)}
                className="flex-1 py-2 border border-gray-300 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleAdd}
                disabled={saving}
                className="flex-1 py-2 bg-purple-600 text-white rounded-lg text-sm font-medium hover:bg-purple-700 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
              >
                {saving ? 'Adding...' : 'Add Product'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
