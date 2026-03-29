import { useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { useAppStore } from '../stores/appStore'
import { getProjects, getAllTasks } from './useWails'

/**
 * Subscribes to backend events for cross-window state synchronization.
 * Each window runs its own JS runtime, so we listen to Go-emitted events
 * and refresh the Zustand store accordingly.
 */
export function useWailsEvents() {
  const { setProjects, setTasks } = useAppStore()

  useEffect(() => {
    const unsubProject = Events.On('project:changed', async () => {
      const projects = await getProjects()
      setProjects(projects || [])
    })

    const unsubTask = Events.On('task:changed', async () => {
      const tasks = await getAllTasks()
      setTasks(tasks || [])
    })

    return () => {
      if (unsubProject) unsubProject()
      if (unsubTask) unsubTask()
    }
  }, [setProjects, setTasks])
}
