/**
 * styled-components TypeScript declaration
 * Extends DefaultTheme with our custom theme type
 */

import 'styled-components';
import { Theme } from './theme';

declare module 'styled-components' {
  // eslint-disable-next-line @typescript-eslint/no-empty-interface
  export interface DefaultTheme extends Theme {}
}
