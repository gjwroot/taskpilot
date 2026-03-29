import { useState, useEffect, useCallback, useRef } from 'react'
import { Events } from '@wailsio/runtime'
import { streamChatWithAI } from './useWails'
import { useAppStore } from '../stores/appStore'

export type StreamStatus = 'idle' | 'streaming' | 'tool_calling' | 'error'

export function useAIStream() {
  const [status, setStatus] = useState<StreamStatus>('idle')
  const [streamingContent, setStreamingContent] = useState('')
  const [toolResults, setToolResults] = useState<{ action: string; success: boolean; message: string }[]>([])
  const contentRef = useRef('')
  const toolResultsRef = useRef<{ action: string; success: boolean; message: string }[]>([])
  const { addChatMessage, selectedProjectId } = useAppStore()

  useEffect(() => {
    const unsubStart = Events.On('ai:stream:start', () => {
      setStatus('streaming')
      setStreamingContent('')
      setToolResults([])
      contentRef.current = ''
      toolResultsRef.current = []
    })

    const unsubChunk = Events.On('ai:stream:chunk', (event: any) => {
      const data = event.data?.[0] || event.data || event
      const text = data.text || ''
      contentRef.current += text
      setStreamingContent(contentRef.current)
    })

    const unsubToolCall = Events.On('ai:stream:tool_call', () => {
      setStatus('tool_calling')
    })

    const unsubToolResult = Events.On('ai:stream:tool_result', (event: any) => {
      const data = event.data?.[0] || event.data || event
      const result = data.result || data
      const tr = {
        action: result.Action || result.action || data.name || '',
        success: result.Success ?? result.success ?? false,
        message: result.Message || result.message || '',
      }
      toolResultsRef.current = [...toolResultsRef.current, tr]
      setToolResults([...toolResultsRef.current])
      setStatus('streaming')
    })

    const unsubEnd = Events.On('ai:stream:end', () => {
      const finalContent = contentRef.current
      const finalToolResults = toolResultsRef.current
      setStatus('idle')
      if (finalContent || finalToolResults.length > 0) {
        addChatMessage({
          role: 'assistant',
          content: finalContent || '',
          toolResults: finalToolResults.length > 0 ? [...finalToolResults] : undefined,
          timestamp: Date.now(),
        })
      }
      setStreamingContent('')
      setToolResults([])
      contentRef.current = ''
      toolResultsRef.current = []
    })

    const unsubError = Events.On('ai:stream:error', (event: any) => {
      const data = event.data?.[0] || event.data || event
      const errorMsg = data.error || 'AI 服务出错'
      setStatus('error')
      addChatMessage({
        role: 'assistant',
        content: errorMsg,
        timestamp: Date.now(),
        isError: true,
      })
      setStreamingContent('')
      contentRef.current = ''
    })

    return () => {
      if (unsubStart) unsubStart()
      if (unsubChunk) unsubChunk()
      if (unsubToolCall) unsubToolCall()
      if (unsubToolResult) unsubToolResult()
      if (unsubEnd) unsubEnd()
      if (unsubError) unsubError()
    }
  }, [addChatMessage])

  const sendMessage = useCallback(async (message: string) => {
    const projectId = selectedProjectId || ''
    addChatMessage({ role: 'user', content: message, timestamp: Date.now() })
    setStatus('streaming')
    setStreamingContent('')
    contentRef.current = ''
    try {
      await streamChatWithAI(message, projectId)
    } catch {
      setStatus('error')
      addChatMessage({
        role: 'assistant',
        content: '抱歉，AI 服务出错了。请检查 API Key 设置。',
        timestamp: Date.now(),
        isError: true,
      })
    }
  }, [addChatMessage, selectedProjectId])

  return { status, streamingContent, toolResults, sendMessage }
}
