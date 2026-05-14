# Grid Component

A beautiful, responsive grid system for displaying cards in a light gray design.

## Usage

```tsx
import Grid from './Grid';
import GridCard from './GridCard';

<Grid columns={3} gap="gap-8">
  <GridCard>
    <h3>Card Title</h3>
    <p>Card content goes here.</p>
  </GridCard>
  <GridCard>
    ...
  </GridCard>
</Grid>
```

- `columns`: Number of columns on desktop (default: 3)
- `gap`: Tailwind gap class (default: gap-6)
- `GridCard` provides a beautiful light gray card with subtle shadow and hover effect.
