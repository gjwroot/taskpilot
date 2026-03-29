import React, { useState, useRef } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { Circle, CheckCircle2, Pencil, Trash2, Clock } from 'lucide-react'
import { FiGitBranch as _FiGitBranch } from 'react-icons/fi'
const FiGitBranch = _FiGitBranch as unknown as React.FC<{ size?: number; className?: string; title?: string }>
import { Task } from '../stores/appStore'
import { useAppStore } from '../stores/appStore'
import { updateTask as updateTaskAPI, deleteTask as deleteTaskAPI, decomposeTask } from '../hooks/useWails'

interface TaskItemProps {
  task: Task
  onEdit: (task: Task) => void
}

const PRIORITY_COLORS: Record<number, { bg: string; text: string; label: string }> = {
  0: { bg: 'bg-red-100/80', text: 'text-red-600', label: 'P0' },
  1: { bg: 'bg-amber-100/80', text: 'text-amber-600', label: 'P1' },
  2: { bg: 'bg-blue-100/80', text: 'text-blue-600', label: 'P2' },
  3: { bg: 'bg-stone-100', text: 'text-stone-500', label: 'P3' },
}

function formatDueDate(dateStr: string): { label: string; overdue: boolean } {
  if (!dateStr) return { label: '', overdue: false }
  const due = new Date(dateStr)
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  due.setHours(0, 0, 0, 0)
  const diffDays = Math.round((due.getTime() - today.getTime()) / (1000 * 60 * 60 * 24))
  const overdue = diffDays < 0
  let label = ''
  if (diffDays === 0) label = '今天到期'
  else if (diffDays === 1) label = '明天到期'
  else if (diffDays === -1) label = '昨天过期'
  else if (overdue) label = `${Math.abs(diffDays)}天前过期`
  else label = `${diffDays}天后到期`
  return { label, overdue }
}

export default function TaskItem({ task, onEdit }: TaskItemProps) {
  const [hovered, setHovered] = useState(false)
  const [decomposing, setDecomposing] = useState(false)
  const [decomposition, setDecomposition] = useState<string | null>(null)
  const [deleteConfirming, setDeleteConfirming] = useState(false)
  const deleteTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const { tasks, setTasks, projects } = useAppStore()

  const project = projects.find(p => p.id === task.projectId)
  const priority = PRIORITY_COLORS[task.priority] ?? PRIORITY_COLORS[3]
  const { label: dueDateLabel, overdue } = formatDueDate(task.dueDate)

  const cycleStatus = async () => {
    const next = task.status === 'todo' ? 'doing' : task.status === 'doing' ? 'done' : 'todo'
    try {
      await updateTaskAPI(task.id, task.title, task.projectId, task.description, next, task.priority, task.dueDate)
      setTasks(tasks.map(t => t.id === task.id ? { ...t, status: next, updatedAt: new Date().toISOString() } : t))
    } catch (err) {
      console.error('Failed to update task status:', err)
    }
  }

  const handleDeleteClick = () => {
    if (!deleteConfirming) {
      setDeleteConfirming(true)
      deleteTimerRef.current = setTimeout(() => setDeleteConfirming(false), 3000)
      return
    }
    if (deleteTimerRef.current) clearTimeout(deleteTimerRef.current)
    setDeleteConfirming(false)
    deleteTaskAPI(task.id)
      .then(() => setTasks(tasks.filter(t => t.id !== task.id)))
      .catch(err => console.error('Failed to delete task:', err))
  }

  const handleDecompose = async () => {
    setDecomposing(true)
    try { setDecomposition(await decomposeTask(task.id)) }
    catch { setDecomposition('任务拆解失败，请检查 API 配置。') }
    finally { setDecomposing(false) }
  }

  return (
    <>
      <motion.div
        layout
        whileHover={{ y: -1 }}
        className="flex items-center gap-3 px-4 py-3 bg-white rounded-xl border border-stone-200/60 transition-all group"
        style={{ boxShadow: hovered ? 'var(--shadow-md)' : 'var(--shadow-xs)' }}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <button
          onClick={cycleStatus}
          title={task.status === 'todo' ? '标记为进行中' : task.status === 'doing' ? '标记为已完成' : '重置为待办'}
          className={`flex-shrink-0 transition-colors ${
            task.status === 'todo' ? 'text-stone-300 hover:text-blue-400' :
            task.status === 'doing' ? 'text-blue-400 hover:text-emerald-500' :
            'text-emerald-500 hover:text-stone-400'
          }`}
        >
          {task.status === 'done' ? (
            <CheckCircle2 size={20} />
          ) : task.status === 'doing' ? (
            <Circle size={20} />
          ) : (
            <Circle size={20} />
          )}
        </button>

        <div className="flex-1 min-w-0">
          <p className={`text-sm font-medium truncate ${task.status === 'done' ? 'line-through text-stone-400' : 'text-stone-800'}`}>
            {task.title}
          </p>
          <div className="flex items-center gap-2 mt-0.5 flex-wrap">
            {project && (
              <span className="text-xs" style={{ color: project.color }}>{project.name}</span>
            )}
            {dueDateLabel && (
              <span className={`flex items-center gap-0.5 text-xs ${overdue ? 'text-red-500' : 'text-stone-400'}`}>
                <Clock size={10} />
                {dueDateLabel}
              </span>
            )}
            {task.tags && task.tags.split(',').filter(Boolean).map((tag, i) => (
              <span
                key={i}
                className="text-[10px] px-1.5 py-0.5 rounded-full bg-violet-100/80 text-violet-600 font-medium"
              >
                {tag.trim()}
              </span>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-1 flex-shrink-0">
          <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded-md ${priority.bg} ${priority.text}`}>
            {priority.label}
          </span>
          <motion.button
            whileHover={{ scale: 1.15 }}
            whileTap={{ scale: 0.9 }}
            onClick={handleDecompose}
            disabled={decomposing}
            className="p-1 text-stone-300 hover:text-purple-500 rounded-md transition-colors disabled:opacity-50"
            title="AI 拆解任务"
          >
            <FiGitBranch size={13} />
          </motion.button>
          <motion.button
            whileHover={{ scale: 1.15 }}
            whileTap={{ scale: 0.9 }}
            onClick={() => onEdit(task)}
            className="p-1 text-stone-300 hover:text-stone-600 rounded-md transition-colors"
          >
            <Pencil size={13} />
          </motion.button>
          <AnimatePresence mode="wait">
            {deleteConfirming ? (
              <motion.button
                key="confirm"
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.8 }}
                transition={{ duration: 0.15 }}
                whileTap={{ scale: 0.9 }}
                onClick={handleDeleteClick}
                className="px-1.5 py-0.5 text-[10px] font-semibold text-white bg-red-500 hover:bg-red-600 rounded-md transition-colors"
              >
                确认删除
              </motion.button>
            ) : (
              <motion.button
                key="trash"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                whileHover={{ scale: 1.15 }}
                whileTap={{ scale: 0.9 }}
                onClick={handleDeleteClick}
                className={`p-1 rounded-md transition-all ${hovered ? 'text-red-400 hover:text-red-600' : 'text-transparent'}`}
              >
                <Trash2 size={13} />
              </motion.button>
            )}
          </AnimatePresence>
        </div>
      </motion.div>

      <AnimatePresence>
        {decomposition && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
            className="ml-10 mr-4 mb-2 overflow-hidden"
          >
            <div className="p-3.5 bg-purple-50/60 border border-purple-100/60 rounded-xl">
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-xs font-semibold text-purple-700">AI 任务拆解</span>
                <button onClick={() => setDecomposition(null)} className="text-xs text-stone-400 hover:text-stone-600 transition-colors">关闭</button>
              </div>
              <div className="text-xs text-stone-600 whitespace-pre-wrap leading-relaxed">{decomposition}</div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  )
}
