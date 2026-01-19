import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export { default as Alert } from "./Alert.vue"
export { default as AlertDescription } from "./AlertDescription.vue"
export { default as AlertTitle } from "./AlertTitle.vue"

export const alertVariants = cva(
  "relative w-full rounded-lg border px-4 py-3 text-sm [&>svg+div]:translate-y-[-3px] [&>svg]:absolute [&>svg]:left-4 [&>svg]:top-4 [&>svg]:text-foreground [&>svg~*]:pl-7",
  {
    variants: {
      variant: {
        default: "bg-background text-foreground",
        destructive:
          "border-destructive/50 text-destructive dark:border-destructive [&>svg]:text-destructive",
        success:
          "border-green-500/50 bg-green-50 text-green-800 dark:bg-green-950 dark:text-green-200 dark:border-green-500/30 [&>svg]:text-green-600 dark:[&>svg]:text-green-400",
        warning:
          "border-yellow-500/50 bg-yellow-50 text-yellow-800 dark:bg-yellow-950 dark:text-yellow-200 dark:border-yellow-500/30 [&>svg]:text-yellow-600 dark:[&>svg]:text-yellow-400",
        info:
          "border-blue-500/50 bg-blue-50 text-blue-800 dark:bg-blue-950 dark:text-blue-200 dark:border-blue-500/30 [&>svg]:text-blue-600 dark:[&>svg]:text-blue-400",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
)

export type AlertVariants = VariantProps<typeof alertVariants>
