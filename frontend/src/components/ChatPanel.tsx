import React, { useRef, useEffect, useState, KeyboardEvent } from 'react'
import { motion } from 'motion/react'
import { X, Trash2, Send, Bot, User } from 'lucide-react'
import { FiClipboard as _FiClipboard, FiTarget as _FiTarget, FiBarChart2 as _FiBarChart2, FiPlusCircle as _FiPlusCircle } from 'react-icons/fi'

type FiIcon = React.FC<{ size?: number; className?: string }>
const FiClipboard = _FiClipboard as unknown as FiIcon
const FiTarget = _FiTarget as unknown as FiIcon
const FiBarChart2 = _FiBarChart2 as unknown as FiIcon
const FiPlusCircle = _FiPlusCircle as unknown as FiIcon
import { useAppStore } from '../stores/appStore'
import { chatWithAI, clearChatHistory as clearChatHistoryAPI } from '../hooks/useWails'

function renderMarkdown(text: string): JSX.Element {
  const lines = text.split('\n')
  const elements: JSX.Element[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]
    if (line.startsWith('```')) {
      const codeLines: string[] = []
      i++
      while (i < lines.length && !lines[i].startsWith('```')) {
        codeLines.push(lines[i])
        i++
      }
      elements.push(
        <pre key={i} className="bg-stone-100 rounded-lg p-2.5 my-1.5 text-xs overflow-x-auto whitespace-pre-wrap font-mono border border-stone-200/50">
          <code>{codeLines.join('\n')}</code>
        </pre>
      )
      i++
      continue
    }
    if (line.match(/^[-*] /)) {
      elements.push(
        <li key={i} className="ml-4 list-disc text-stone-600">
          {renderInline(line.slice(2))}
        </li>
      )
      i++
      continue
    }
    if (line.trim() === '') {
      elements.push(<div key={i} className="h-1" />)
      i++
      continue
    }
    elements.push(
      <p key={i} className="leading-relaxed">
        {renderInline(line)}
      </p>
    )
    i++
  }
  return <>{elements}</>
}

function renderInline(text: string): JSX.Element {
  const parts: JSX.Element[] = []
  const regex = /(\*\*(.+?)\*\*|`([^`]+)`)/g
  let lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push(<span key={lastIndex}>{text.slice(lastIndex, match.index)}</span>)
    }
    if (match[0].startsWith('**')) {
      parts.push(<strong key={match.index} className="font-semibold text-stone-800">{match[2]}</strong>)
    } else {
      parts.push(
        <code key={match.index} className="bg-stone-100 rounded px-1 py-0.5 text-xs font-mono text-indigo-600">
          {match[3]}
        </code>
      )
    }
    lastIndex = match.index + match[0].length
  }
  if (lastIndex < text.length) {
    parts.push(<span key={lastIndex}>{text.slice(lastIndex)}</span>)
  }
  return <>{parts}</>
}

function LoadingDots() {
  return (
    <div className="flex items-center gap-1 py-1">
      <span className="loading-dot w-1.5 h-1.5 rounded-full bg-stone-400" style={{ animationDelay: '0ms' }} />
      <span className="loading-dot w-1.5 h-1.5 rounded-full bg-stone-400" style={{ animationDelay: '150ms' }} />
      <span className="loading-dot w-1.5 h-1.5 rounded-full bg-stone-400" style={{ animationDelay: '300ms' }} />
    </div>
  )
}

const QUICK_PROMPTS: { icon: FiIcon; label: string; text: string }[] = [
  { icon: FiClipboard, label: '总结今日进度', text: '总结今日进度' },
  { icon: FiTarget, label: '帮我规划今天的任务', text: '帮我规划今天的任务' },
  { icon: FiBarChart2, label: '本周完成了哪些任务？', text: '本周完成了哪些任务？' },
  { icon: FiPlusCircle, label: '创建一个新任务', text: '创建一个新任务' },
]

export default function ChatPanel({ standalone = false }: { standalone?: boolean }) {
  const { chatMessages, addChatMessage, clearChatMessages, toggleChatPanel } = useAppStore()
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [chatMessages, loading])

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value)
    const el = e.target
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 96) + 'px'
  }

  const handleSend = async (message: string) => {
    const trimmed = message.trim()
    if (!trimmed || loading) return
    setInput('')
    if (textareaRef.current) textareaRef.current.style.height = 'auto'
    addChatMessage({ role: 'user', content: trimmed, timestamp: Date.now() })
    setLoading(true)
    try {
      const response = await chatWithAI(trimmed)
      addChatMessage({
        role: 'assistant',
        content: response.text,
        toolResults: response.toolCalls,
        timestamp: Date.now(),
      })
    } catch (_err) {
      addChatMessage({
        role: 'assistant',
        content: '抱歉，AI 服务出错了。请检查 API Key 设置。',
        timestamp: Date.now(),
      })
    } finally {
      setLoading(false)
    }
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend(input)
    }
  }

  return (
    <div className="flex flex-col w-full h-full bg-white/80 backdrop-blur-xl border-l border-stone-200/60 flex-shrink-0">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-stone-100 glass">
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded-lg bg-indigo-500/10 flex items-center justify-center">
            <Bot size={14} className="text-indigo-500" />
          </div>
          <span className="font-semibold text-stone-800 text-sm">AI 助手</span>
        </div>
        <div className="flex items-center gap-0.5">
          <button
            onClick={clearChatMessages}
            className="p-1.5 rounded-lg hover:bg-stone-100 text-stone-400 hover:text-stone-600 transition-colors"
            title="清空对话"
          >
            <Trash2 size={14} />
          </button>
          {!standalone && (
            <button
              onClick={toggleChatPanel}
              className="p-1.5 rounded-lg hover:bg-stone-100 text-stone-400 hover:text-stone-600 transition-colors"
              title="关闭"
            >
              <X size={14} />
            </button>
          )}
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
        {chatMessages.length === 0 && !loading ? (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col items-center gap-2 py-8 text-stone-400">
              <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-indigo-50 to-purple-50 flex items-center justify-center">
                <Bot size={24} className="text-indigo-400" />
              </div>
              <p className="text-sm mt-1">有什么可以帮你的吗？</p>
            </div>
            <div className="flex flex-col gap-2">
              {QUICK_PROMPTS.map((p, idx) => (
                <motion.button
                  key={p.text}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: idx * 0.05, duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
                  whileHover={{ x: 4 }}
                  onClick={() => handleSend(p.text)}
                  className="flex items-center gap-2.5 text-left px-3.5 py-2.5 rounded-xl border border-stone-200/80 text-sm text-stone-600 hover:bg-indigo-50/50 hover:border-indigo-200 hover:text-indigo-700 transition-colors"
                >
                  <p.icon size={14} className="flex-shrink-0 opacity-60" />
                  {p.label}
                </motion.button>
              ))}
            </div>
          </div>
        ) : (
          <>
            {chatMessages.map((msg, idx) => (
              <motion.div
                key={idx}
                initial={{ opacity: 0, y: 10, scale: 0.97 }}
                animate={{ opacity: 1, y: 0, scale: 1 }}
                transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
                className={`flex gap-2 ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
              >
                {msg.role === 'assistant' && (
                  <div className="flex-shrink-0 w-6 h-6 rounded-lg bg-indigo-50 flex items-center justify-center mt-0.5">
                    <Bot size={12} className="text-indigo-500" />
                  </div>
                )}
                <div className="max-w-[78%] flex flex-col gap-1.5">
                  <div
                    className={`px-3 py-2 rounded-2xl text-[13px] leading-relaxed ${
                      msg.role === 'user'
                        ? 'bg-indigo-500 text-white rounded-br-md'
                        : 'bg-stone-100/80 text-stone-700 rounded-bl-md'
                    }`}
                  >
                    {msg.role === 'assistant' ? renderMarkdown(msg.content) : msg.content}
                  </div>
                  {msg.toolResults && msg.toolResults.length > 0 && (
                    <div className="flex flex-col gap-1">
                      {msg.toolResults.map((tr, ti) => (
                        <div
                          key={ti}
                          className={`flex items-start gap-1.5 px-2.5 py-1.5 rounded-lg text-xs border ${
                            tr.success
                              ? 'border-emerald-200/60 bg-emerald-50/50 text-emerald-700'
                              : 'border-red-200/60 bg-red-50/50 text-red-700'
                          }`}
                        >
                          <span className="mt-0.5 flex-shrink-0">{tr.success ? '✓' : '✗'}</span>
                          <div>
                            <span className="font-medium">{tr.action}</span>
                            {tr.message && <span className="ml-1 opacity-70">— {tr.message}</span>}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
                {msg.role === 'user' && (
                  <div className="flex-shrink-0 w-6 h-6 rounded-lg bg-indigo-500 flex items-center justify-center mt-0.5">
                    <User size={12} className="text-white" />
                  </div>
                )}
              </motion.div>
            ))}
            {loading && (
              <div className="flex gap-2 justify-start">
                <div className="flex-shrink-0 w-6 h-6 rounded-lg bg-indigo-50 flex items-center justify-center mt-0.5">
                  <Bot size={12} className="text-indigo-500" />
                </div>
                <div className="bg-stone-100/80 px-3 py-2 rounded-2xl rounded-bl-md">
                  <LoadingDots />
                </div>
              </div>
            )}
          </>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="px-4 py-3 border-t border-stone-100 glass">
        <div className="flex items-end gap-2 bg-stone-50/80 border border-stone-200/60 rounded-xl px-3 py-2 focus-within:border-indigo-300 focus-within:shadow-[0_0_0_3px_rgba(99,102,241,0.08)] transition-all">
          <textarea
            ref={textareaRef}
            value={input}
            onChange={handleInputChange}
            onKeyDown={handleKeyDown}
            disabled={loading}
            placeholder="输入消息… (Enter 发送)"
            rows={1}
            className="flex-1 bg-transparent resize-none outline-none text-sm text-stone-800 placeholder-stone-400 leading-6 max-h-24 disabled:opacity-50"
          />
          <motion.button
            whileHover={{ scale: 1.05 }}
            whileTap={{ scale: 0.95 }}
            onClick={() => handleSend(input)}
            disabled={loading || !input.trim()}
            className="flex-shrink-0 p-1.5 rounded-lg bg-indigo-500 text-white hover:bg-indigo-600 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            <Send size={13} />
          </motion.button>
        </div>
        <p className="text-[10px] text-stone-400 mt-1.5 text-center tracking-wide">Powered by Claude AI</p>
      </div>
    </div>
  )
}
