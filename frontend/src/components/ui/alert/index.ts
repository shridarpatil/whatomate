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
          "border-destructive/50 text-destructive light:border-destructive [&>svg]:text-destructive",
        // Dark-first: dark mode is default, light mode uses light: prefix
        success:
          "border-green-500/30 bg-green-950 text-green-200 [&>svg]:text-green-400 light:border-green-500/50 light:bg-green-50 light:text-green-800 light:[&>svg]:text-green-600",
        warning:
          "border-yellow-500/30 bg-yellow-950 text-yellow-200 [&>svg]:text-yellow-400 light:border-yellow-500/50 light:bg-yellow-50 light:text-yellow-800 light:[&>svg]:text-yellow-600",
        info:
          "border-blue-500/30 bg-blue-950 text-blue-200 [&>svg]:text-blue-400 light:border-blue-500/50 light:bg-blue-50 light:text-blue-800 light:[&>svg]:text-blue-600",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
)

export type AlertVariants = VariantProps<typeof alertVariants>
