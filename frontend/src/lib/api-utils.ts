/**
 * API utility functions for handling responses and errors consistently
 */

import { isHttpError, type HttpResponse } from './http'

/**
 * Unwraps API response data, handling both nested and flat response structures.
 * API responses may come as { data: { data: T } } or { data: T }
 */
export function unwrapResponse<T>(response: HttpResponse): T {
  const data = response.data
  // Handle nested data structure: { data: { data: T } }
  if (data && typeof data === 'object' && 'data' in data) {
    return data.data as T
  }
  // Return as-is for flat structure
  return data as T
}

/**
 * Unwraps a list response, handling various API response formats.
 * Supports: { data: { data: { items: T[] } } }, { data: { items: T[] } }, { data: T[] }
 */
export function unwrapListResponse<T>(
  response: HttpResponse,
  key: string
): T[] {
  const data = response.data

  // Handle deeply nested: { data: { data: { [key]: T[] } } }
  if (data?.data?.[key]) {
    return data.data[key] as T[]
  }

  // Handle nested: { data: { [key]: T[] } }
  if (data?.[key]) {
    return data[key] as T[]
  }

  // Handle array directly: { data: T[] }
  if (Array.isArray(data?.data)) {
    return data.data as T[]
  }

  if (Array.isArray(data)) {
    return data as T[]
  }

  return []
}

/**
 * Unwraps a single item response, handling various API response formats.
 */
export function unwrapItemResponse<T>(
  response: HttpResponse,
  key?: string
): T {
  const data = response.data

  if (key) {
    // Handle deeply nested: { data: { data: { [key]: T } } }
    if (data?.data?.[key]) {
      return data.data[key] as T
    }

    // Handle nested: { data: { [key]: T } }
    if (data?.[key]) {
      return data[key] as T
    }
  }

  // Handle nested without key: { data: { data: T } }
  if (data?.data && typeof data.data === 'object' && !Array.isArray(data.data)) {
    return data.data as T
  }

  // Return data directly
  return data as T
}

/**
 * Extracts error message from various error formats.
 * Handles HTTP client errors, standard errors, and string errors.
 */
export function getErrorMessage(error: unknown, defaultMessage = 'An error occurred'): string {
  if (!error) {
    return defaultMessage
  }

  // Handle HTTP error with response
  if (isHttpError(error)) {
    const responseMessage = error.response?.data?.message
    if (responseMessage && typeof responseMessage === 'string') {
      return responseMessage
    }

    // Handle error array in response
    const errors = error.response?.data?.errors
    if (Array.isArray(errors) && errors.length > 0) {
      return errors[0].message || errors[0] || defaultMessage
    }
  }

  // Handle standard Error object
  if (error instanceof Error) {
    return error.message || defaultMessage
  }

  // Handle string error
  if (typeof error === 'string') {
    return error
  }

  return defaultMessage
}
