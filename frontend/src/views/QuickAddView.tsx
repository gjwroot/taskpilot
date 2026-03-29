import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { createTask, getProjects } from '../hooks/useWails'
import type { Project } from '../stores/appStore'

export default function QuickAddView() {
  const [title, setTitle] = useState('')
  const [projectId, setProjectId] = useState('')
  const [priority, setPriority] = useState(2)
  const [dueDate, setDueDate] = useState('')
  const [projects, setProjects] = useState<Project[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [success, setSuccess] = useState(false)

  useEffect(() => {
    getProjects().then((p) => {
      setProjects(p || [])
      if (p && p.length > 0) setProjectId(p[0].id)
    })
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) return
    setSubmitting(true)
    try {
      await createTask(title.trim(), projectId, '', priority, dueDate)
      setSuccess(true)
      setTitle('')
      setDueDate('')
      setTimeout(() => setSuccess(false), 1500)
    } catch (err) {
      console.error('Failed to create task:', err)
    } finally {
      setSubmitting(false)
    }
  }

  const priorityLabels = ['P0 紧急', 'P1 高', 'P2 中', 'P3 低']
  const priorityColors = ['#ef4444', '#f97316', '#3b82f6', '#9ca3af']

  return (
    <div className="h-screen w-screen p-5 flex flex-col" style={{
      background: 'rgba(255,255,255,0.85)',
      backdropFilter: 'blur(40px)',
      WebkitBackdropFilter: 'blur(40px)',
      borderRadius: '12px',
      overflow: 'hidden',
    }}>
      <div style={{ WebkitAppRegion: 'drag' } as React.CSSProperties} className="h-4 flex-shrink-0" />
      <h2 className="text-base font-semibold mb-3" style={{ color: 'var(--text-primary, #111)' }}>
        快速添加任务
      </h2>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3 flex-1">
        <input
          type="text"
          placeholder="任务标题..."
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          autoFocus
          className="px-3 py-2 rounded-lg border text-sm outline-none focus:ring-2 focus:ring-blue-400"
          style={{
            background: 'var(--bg-secondary, #f9fafb)',
            borderColor: 'var(--border, #e5e7eb)',
            color: 'var(--text-primary, #111)',
          }}
        />

        <select
          value={projectId}
          onChange={(e) => setProjectId(e.target.value)}
          className="px-3 py-2 rounded-lg border text-sm outline-none"
          style={{
            background: 'var(--bg-secondary, #f9fafb)',
            borderColor: 'var(--border, #e5e7eb)',
            color: 'var(--text-primary, #111)',
          }}
        >
          <option value="">无项目</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>

        <div className="flex gap-2">
          {priorityLabels.map((label, i) => (
            <button
              key={i}
              type="button"
              onClick={() => setPriority(i)}
              className="flex-1 px-2 py-1.5 rounded-lg text-xs font-medium transition-all"
              style={{
                background: priority === i ? priorityColors[i] : 'var(--bg-secondary, #f3f4f6)',
                color: priority === i ? '#fff' : 'var(--text-secondary, #6b7280)',
                border: `1px solid ${priority === i ? priorityColors[i] : 'var(--border, #e5e7eb)'}`,
              }}
            >
              {label}
            </button>
          ))}
        </div>

        <input
          type="date"
          value={dueDate}
          onChange={(e) => setDueDate(e.target.value)}
          className="px-3 py-2 rounded-lg border text-sm outline-none"
          style={{
            background: 'var(--bg-secondary, #f9fafb)',
            borderColor: 'var(--border, #e5e7eb)',
            color: 'var(--text-primary, #111)',
          }}
        />

        <div className="flex-1" />

        <button
          type="submit"
          disabled={!title.trim() || submitting}
          className="w-full py-2.5 rounded-lg text-sm font-medium text-white transition-all"
          style={{
            background: success ? '#22c55e' : '#3b82f6',
            opacity: !title.trim() || submitting ? 0.5 : 1,
          }}
        >
          {success ? '已添加 ✓' : submitting ? '添加中...' : '添加任务'}
        </button>
      </form>
    </div>
  )
}
