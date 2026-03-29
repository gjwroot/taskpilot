import { ProjectService } from '../../bindings/taskpilot/services'
import { TaskService } from '../../bindings/taskpilot/services'
import { AIService } from '../../bindings/taskpilot/services'
import { ConfigService } from '../../bindings/taskpilot/services'

import type { Project, Task } from '../stores/appStore'

// ---- 项目相关 ----

export async function getProjects(): Promise<Project[]> {
  return await ProjectService.GetProjects()
}

export async function createProject(name: string, description: string, color: string): Promise<Project> {
  const result = await ProjectService.CreateProject(name, description, color)
  return result!
}

export async function updateProject(id: string, name: string, description: string, color: string): Promise<Project> {
  await ProjectService.UpdateProject(id, name, description, color)
  return { id, name, description, color, createdAt: '', updatedAt: new Date().toISOString() }
}

export async function deleteProject(id: string): Promise<void> {
  await ProjectService.DeleteProject(id)
}

// ---- 任务相关 ----

export async function getAllTasks(): Promise<Task[]> {
  return await TaskService.GetAllTasks()
}

export async function getTasksByProject(projectId: string): Promise<Task[]> {
  return await TaskService.GetTasksByProject(projectId)
}

export async function getTodayTasks(): Promise<Task[]> {
  return await TaskService.GetTodayTasks()
}

export async function createTask(
  title: string,
  projectId: string,
  description: string,
  priority: number,
  dueDate: string
): Promise<Task> {
  const result = await TaskService.CreateTask(title, projectId, description, priority, dueDate)
  return result!
}

export async function updateTask(
  id: string,
  title: string,
  projectId: string,
  description: string,
  status: string,
  priority: number,
  dueDate: string
): Promise<void> {
  await TaskService.UpdateTask(id, title, projectId, description, status, priority, dueDate)
}

export async function deleteTask(id: string): Promise<void> {
  await TaskService.DeleteTask(id)
}

// ---- AI 相关 ----

export interface ChatResponse {
  text: string
  toolCalls: { action: string; success: boolean; message: string }[]
}

export async function chatWithAI(message: string): Promise<ChatResponse> {
  const result = await AIService.ChatWithAI(message)
  return result!
}

export async function getDailySummary(): Promise<string> {
  return await AIService.GetDailySummary()
}

export async function clearChatHistory(): Promise<void> {
  await AIService.ClearChatHistory()
}

// ---- AI 高级功能 ----

export async function smartSuggestTasks(projectId: string): Promise<string> {
  return await AIService.SmartSuggestTasks(projectId)
}

export async function decomposeTask(taskId: string): Promise<string> {
  return await AIService.DecomposeTask(taskId)
}

export async function prioritizeTasks(projectId: string): Promise<string> {
  return await AIService.PrioritizeTasks(projectId)
}

export async function generateWeeklyReport(): Promise<string> {
  return await AIService.GenerateWeeklyReport()
}

// ---- 设置相关 ----

export interface AIConfigData {
  apiKey: string
  baseURL: string
  model: string
}

export async function getAPIKey(): Promise<string> {
  return await ConfigService.GetAPIKey()
}

export async function saveAPIKey(key: string): Promise<void> {
  await ConfigService.SaveAPIKey(key)
}

export async function getAIConfig(): Promise<AIConfigData> {
  const result = await ConfigService.GetAIConfig()
  return result!
}

export async function saveAIConfig(apiKey: string, baseURL: string, model: string): Promise<void> {
  await ConfigService.SaveAIConfig(apiKey, baseURL, model)
}

export async function testAIConnection(): Promise<void> {
  await AIService.TestAIConnection()
}
