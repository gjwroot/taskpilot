import { Streamdown } from 'streamdown'
import 'streamdown/styles.css'

interface MarkdownRendererProps {
  content: string
  isStreaming?: boolean
}

export default function MarkdownRenderer({ content, isStreaming = false }: MarkdownRendererProps) {
  return (
    <Streamdown isAnimating={isStreaming}>
      {content}
    </Streamdown>
  )
}
