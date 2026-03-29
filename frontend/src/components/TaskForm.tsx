import { useState, useEffect } from 'react'
import { motion } from 'motion/react'
import { X } from 'lucide-react'
import { Task, useAppStore } from '../stores/appStore'
import { createTask as createTaskAPI, updateTask as updateTaskAPI } from '../hooks/useWails'

interface TaskFormProps {
  task?: Task
  onClose: () => void
  onSave: () => void
}

const PRIORITIES = [
  { value: 0, label: 'P0', color: 'bg-red-500 text-white border-red-500', ring: 'ring-red-200' },
  { value: 1, label: 'P1', color: 'bg-amber-500 text-white border-amber-500', ring: 'ring-amber-200' },
  { value: 2, label: 'P2', color: 'bg-blue-500 text-white border-blue-500', ring: 'ring-blue-200' },
  { value: 3, label: 'P3', color: 'bg-stone-400 text-white border-stone-400', ring: 'ring-stone-200' },
]

const STATUSES = [
  { value: 'todo', label: '待办', activeClass: 'bg-stone-600 text-white border-stone-600' },
  { value: 'doing', label: '进行中', activeClass: 'bg-blue-500 text-white border-blue-500' },
  { value: 'done', label: '已完成', activeClass: 'bg-emerald-500 text-white border-emerald-500' },
]

export default function TaskForm({ task, onClose, onSave }: TaskFormProps) {
  const { projects, tasks, setTasks, selectedProjectId } = useAppStore()

  const [title, setTitle] = useState(task?.title ?? '')
  const [description, setDescription] = useState(task?.description ?? '')
  const [projectId, setProjectId] = useState(task?.projectId ?? selectedProjectId ?? projects[0]?.id ?? '')
  const [priority, setPriority] = useState(task?.priority ?? 2)
  const [status, setStatus] = useState(task?.status ?? 'todo')
  const [dueDate, setDueDate] = useState(task?.dueDate ?? '')
  const [error, setError] = useState('')

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [onClose])

  const handleSave = async () => {
    if (!title.trim()) { setError('标题不能为空'); return }
    setError('')
    try {
      if (task) {
        await updateTaskAPI(task.id, title, projectId, description, status, priority, dueDate)
        setTasks(tasks.map(t => t.id === task.id ? { ...t, title, projectId, description, priority, status, dueDate, updatedAt: new Date().toISOString() } : t))
      } else {
        const newTask = await createTaskAPI(title, projectId, description, priority, dueDate)
        setTasks([...tasks, newTask])
      }
      onSave()
    } catch { setError('保存失败，请重试') }
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.2 }}
      className="fixed inset-0 flex items-center justify-center z-50"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div className="absolute inset-0 bg-black/30 backdrop-blur-sm" />
      <motion.div
        initial={{ opacity: 0, scale: 0.95, y: 10 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.95, y: 10 }}
        transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
        className="relative bg-white rounded-2xl w-full max-w-md mx-4 overflow-hidden"
        style={{ boxShadow: 'var(--shadow-xl)' }}
      >
          <div className="flex items-center justify-between px-6 py-4 border-b border-stone-100">
            <h2 className="text-base font-semibold text-stone-800">{task ? '编辑任务' : '新建任务'}</h2>
            <button onClick={onClose} className="text-stone-400 hover:text-stone-600 transition-colors p-1 rounded-lg hover:bg-stone-100">
              <X size={16} />
            </button>
          </div>

          <div className="px-6 py-4 space-y-4">
            <div>
              <label className="block text-xs font-medium text-stone-600 mb-1.5">标题 <span className="text-red-500">*</span></label>
              <input
                type="text"
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder="输入任务标题..."
                className="w-full px-3.5 py-2.5 text-sm border border-stone-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-400 transition-all"
                autoFocus
              />
              {error && <p className="text-xs text-red-500 mt-1">{error}</p>}
            </div>

            <div>
              <label className="block text-xs font-medium text-stone-600 mb-1.5">描述</label>
              <textarea
                value={description}
                onChange={e => setDescription(e.target.value)}
                placeholder="添加描述（可选）..."
                rows={3}
                className="w-full px-3.5 py-2.5 text-sm border border-stone-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-400 resize-none transition-all"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-stone-600 mb-1.5">项目</label>
              <select
                value={projectId}
                onChange={e => setProjectId(e.target.value)}
                className="w-full px-3.5 py-2.5 text-sm border border-stone-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-400 bg-white transition-all"
              >
                {projects.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-stone-600 mb-2">优先级</label>
              <div className="flex gap-2">
                {PRIORITIES.map(p => (
                  <button
                    key={p.value}
                    onClick={() => setPriority(p.value)}
                    className={`flex-1 py-1.5 text-xs font-semibold rounded-lg border transition-all ${
                      priority === p.value ? `${p.color} ring-2 ${p.ring}` : 'border-stone-200 text-stone-500 hover:border-stone-300'
                    }`}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-stone-600 mb-2">状态</label>
              <div className="flex gap-2">
                {STATUSES.map(s => (
                  <button
                    key={s.value}
                    onClick={() => setStatus(s.value)}
                    className={`flex-1 py-1.5 text-xs font-semibold rounded-lg border transition-all ${
                      status === s.value ? s.activeClass : 'border-stone-200 text-stone-500 hover:border-stone-300'
                    }`}
                  >
                    {s.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-stone-600 mb-1.5">截止日期</label>
              <input
                type="date"
                value={dueDate}
                onChange={e => setDueDate(e.target.value)}
                className="w-full px-3.5 py-2.5 text-sm border border-stone-200 rounded-xl focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-400 transition-all"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-stone-600 mb-1.5">标签 <span className="text-stone-400 font-normal">（AI 自动生成）</span></label>
              <div className="flex flex-wrap gap-1.5">
                {(task?.tags || '').split(',').filter(Boolean).map((tag, i) => (
                  <span
                    key={i}
                    className="text-xs px-2 py-1 rounded-full bg-violet-100/80 text-violet-600 font-medium"
                  >
                    {tag.trim()}
                  </span>
                ))}
                {!(task?.tags) && <span className="text-xs text-stone-400">保存后 AI 将自动生成标签</span>}
              </div>
            </div>
          </div>

          <div className="flex gap-3 px-6 py-4 border-t border-stone-100">
            <motion.button
              whileTap={{ scale: 0.98 }}
              onClick={onClose}
              className="flex-1 py-2.5 text-sm font-medium text-stone-600 bg-stone-100 hover:bg-stone-200 rounded-xl transition-colors"
            >
              取消
            </motion.button>
            <motion.button
              whileHover={{ scale: 1.01 }}
              whileTap={{ scale: 0.98 }}
              onClick={handleSave}
              className="flex-1 py-2.5 text-sm font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-xl transition-colors"
            >
              {task ? '保存修改' : '创建任务'}
            </motion.button>
          </div>
        </motion.div>
    </motion.div>
  )
}
