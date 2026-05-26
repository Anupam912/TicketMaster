/**
 * UI Components Barrel Export
 * 
 * A "barrel" file re-exports components from a single place.
 * This makes imports cleaner:
 * 
 * Instead of:
 *   import { Button } from '@/components/ui/Button';
 *   import { Input } from '@/components/ui/Input';
 * 
 * We can do:
 *   import { Button, Input } from '@/components/ui';
 */

export { Button } from './Button';
export { Input } from './Input';
export { 
  Card, 
  CardHeader, 
  CardTitle, 
  CardDescription, 
  CardContent, 
  CardFooter 
} from './Card';
