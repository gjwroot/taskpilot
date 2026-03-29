import { create } from 'zustand'

export interface Project {
  id: string
  name: string
  description: string
  color: string
  createdAt: string
  updatedAt: string
}

export interface Task {
  id: string
  projectId: string
  title: string
  description: string
  status: string  // todo, doing, done
  priority: number // 0-3
  dueDate: string
  tags: string       // 逗号分隔标签
  createdAt: string
  updatedAt: string
}

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  toolResults?: { action: string; success: boolean; message: string }[]
  timestamp: number
  isError?: boolean
}

interface AppState {
  // 项目
  projects: Project[]
  selectedProjectId: string | null
  // 任务
  tasks: Task[]
  // 视图
  currentView: 'project' | 'today' | 'settings' | 'logs'
  // AI 面板
  showChatPanel: boolean
  chatMessages: ChatMessage[]
  // 操作
  setProjects: (projects: Project[]) => void
  setSelectedProjectId: (id: string | null) => void
  setTasks: (tasks: Task[]) => void
  setCurrentView: (view: 'project' | 'today' | 'settings' | 'logs') => void
  toggleChatPanel: () => void
  addChatMessage: (msg: ChatMessage) => void
  clearChatMessages: () => void
}

export const useAppStore = create<AppState>((set) => ({
  projects: [],
  selectedProjectId: null,
  tasks: [],
  currentView: 'today',
  showChatPanel: false,
  chatMessages: [],

  setProjects: (projects) => set({ projects }),
  setSelectedProjectId: (id) => set({ selectedProjectId: id, currentView: 'project' }),
  setTasks: (tasks) => set({ tasks }),
  setCurrentView: (view) => set({ currentView: view }),
  toggleChatPanel: () => set((state) => ({ showChatPanel: !state.showChatPanel })),
  addChatMessage: (msg) => set((state) => ({ chatMessages: [...state.chatMessages, msg] })),
  clearChatMessages: () => set({ chatMessages: [] }),
}))
