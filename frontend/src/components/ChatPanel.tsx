import React, { useRef, useEffect, useState, KeyboardEvent } from 'react'
import { motion } from 'motion/react'
import { X, Trash2, Send, Bot, User, Loader2, AlertCircle } from 'lucide-react'
import { FiClipboard as _FiClipboard, FiTarget as _FiTarget, FiBarChart2 as _FiBarChart2, FiPlusCircle as _FiPlusCircle } from 'react-icons/fi'

type FiIcon = React.FC<{ size?: number; className?: string }>
const FiClipboard = _FiClipboard as unknown as FiIcon
const FiTarget = _FiTarget as unknown as FiIcon
const FiBarChart2 = _FiBarChart2 as unknown as FiIcon
const FiPlusCircle = _FiPlusCircle as unknown as FiIcon
import { useAppStore } from '../stores/appStore'
import { clearChatHistory as clearChatHistoryAPI } from '../hooks/useWails'
import { useAIStream } from '../hooks/useAIStream'
import MarkdownRenderer from './MarkdownRenderer'
import ProactiveSuggestions from './ProactiveSuggestions'

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
  const { chatMessages, clearChatMessages, toggleChatPanel } = useAppStore()
  const { status, streamingContent, toolResults, sendMessage } = useAIStream()
  const isStreaming = status === 'streaming' || status === 'tool_calling'
  const [input, setInput] = useState('')
  const [confirmClear, setConfirmClear] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const confirmClearTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [chatMessages, isStreaming, streamingContent])

  useEffect(() => {
    return () => {
      if (confirmClearTimerRef.current) clearTimeout(confirmClearTimerRef.current)
    }
  }, [])

  const handleClear = async () => {
    if (!confirmClear) {
      setConfirmClear(true)
      confirmClearTimerRef.current = setTimeout(() => setConfirmClear(false), 3000)
      return
    }
    if (confirmClearTimerRef.current) clearTimeout(confirmClearTimerRef.current)
    setConfirmClear(false)
    try {
      await clearChatHistoryAPI()
    } catch (_e) {
      // ignore backend error, still clear frontend
    }
    clearChatMessages()
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value)
    const el = e.target
    el.style.height = 'auto'
    el.style.height = Math.min(el.scrollHeight, 96) + 'px'
  }

  const handleSend = async (message: string) => {
    const trimmed = message.trim()
    if (!trimmed || isStreaming) return
    setInput('')
    if (textareaRef.current) textareaRef.current.style.height = 'auto'
    await sendMessage(trimmed)
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
          {status === 'tool_calling' && (
            <span className="text-xs text-amber-500 animate-pulse">执行中...</span>
          )}
          {status === 'streaming' && (
            <span className="text-xs text-indigo-400 animate-pulse">思考中...</span>
          )}
        </div>
        <div className="flex items-center gap-0.5">
          <button
            onClick={handleClear}
            className={`p-1.5 rounded-lg transition-colors text-xs flex items-center gap-1 ${
              confirmClear
                ? 'bg-red-50 text-red-500 hover:bg-red-100 px-2'
                : 'hover:bg-stone-100 text-stone-400 hover:text-stone-600'
            }`}
            title={confirmClear ? '再次点击确认清空' : '清空对话'}
          >
            <Trash2 size={14} />
            {confirmClear && <span className="font-medium">确认清空?</span>}
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
        {chatMessages.length === 0 && !isStreaming ? (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col items-center gap-2 py-8 text-stone-400">
              <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-indigo-50 to-purple-50 flex items-center justify-center">
                <Bot size={24} className="text-indigo-400" />
              </div>
              <p className="text-sm mt-1">有什么可以帮你的吗？</p>
            </div>
            <ProactiveSuggestions onSend={handleSend} />
            <div className="flex flex-col gap-2">
              {QUICK_PROMPTS.map((p, idx) => (
                <motion.button
                  key={p.text}
                  initial={{ y: 8 }}
                  animate={{ y: 0 }}
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
                        : msg.isError
                        ? 'bg-red-50/80 text-red-700 border border-red-200/60 rounded-bl-md'
                        : 'bg-stone-100/80 text-stone-700 rounded-bl-md'
                    }`}
                  >
                    {msg.role === 'assistant' ? (
                      msg.isError ? (
                        <div className="flex items-start gap-1.5">
                          <AlertCircle size={13} className="flex-shrink-0 mt-0.5" />
                          <span>{msg.content}</span>
                        </div>
                      ) : (
                        <MarkdownRenderer content={msg.content} isStreaming={false} />
                      )
                    ) : msg.content}
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
            {isStreaming && (
              <div className="flex gap-2 justify-start">
                <div className="flex-shrink-0 w-6 h-6 rounded-lg bg-indigo-50 flex items-center justify-center mt-0.5">
                  <Bot size={12} className="text-indigo-500" />
                </div>
                <div className="max-w-[78%] flex flex-col gap-1.5">
                  <div className="bg-stone-100/80 px-3 py-2 rounded-2xl rounded-bl-md text-[13px] text-stone-700">
                    {streamingContent ? (
                      <MarkdownRenderer content={streamingContent} isStreaming={true} />
                    ) : (
                      <LoadingDots />
                    )}
                  </div>
                  {toolResults.length > 0 && (
                    <div className="flex flex-col gap-1">
                      {toolResults.map((tr, ti) => (
                        <div key={ti} className={`flex items-start gap-1.5 px-2.5 py-1.5 rounded-lg text-xs border ${
                          tr.success ? 'border-emerald-200/60 bg-emerald-50/50 text-emerald-700' : 'border-red-200/60 bg-red-50/50 text-red-700'
                        }`}>
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
            disabled={isStreaming}
            placeholder="输入消息… (Enter 发送)"
            rows={1}
            className="flex-1 bg-transparent resize-none outline-none text-sm text-stone-800 placeholder-stone-400 leading-6 max-h-24 disabled:opacity-50"
          />
          <motion.button
            whileHover={!isStreaming ? { scale: 1.05 } : {}}
            whileTap={!isStreaming ? { scale: 0.95 } : {}}
            onClick={() => handleSend(input)}
            disabled={isStreaming || !input.trim()}
            className="flex-shrink-0 p-1.5 rounded-lg bg-indigo-500 text-white hover:bg-indigo-600 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
          >
            {isStreaming ? <Loader2 size={13} className="animate-spin" /> : <Send size={13} />}
          </motion.button>
        </div>
        <p className="text-[10px] text-stone-400 mt-1.5 text-center tracking-wide">Powered by Claude AI</p>
      </div>
    </div>
  )
}
