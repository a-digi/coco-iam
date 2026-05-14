const fs = require('fs');
const path = require('path');

function walk(dir) {
    let results = [];
    const list = fs.readdirSync(dir);
    list.forEach(function(file) {
        file = path.join(dir, file);
        const stat = fs.statSync(file);
        if (stat && stat.isDirectory()) { 
            results = results.concat(walk(file));
        } else if (file.endsWith('.tsx') || file.endsWith('.ts')) {
            results.push(file);
        }
    });
    return results;
}

const files = walk('./src');
files.forEach(file => {
    let content = fs.readFileSync(file, 'utf8');
    let newContent = content.replace(/dark:([a-zA-Z0-9\-]+)-gray-([a-zA-Z0-9]+)/g, 'dark:$1-zinc-$2');
    
    // Specifically upgrade the main background to zinc-950 for deeper contrast
    if (file.includes('LayoutContext.tsx')) {
        newContent = newContent.replace('dark:bg-zinc-900', 'dark:bg-zinc-950');
    }
    
    if (content !== newContent) {
        fs.writeFileSync(file, newContent);
        console.log(`Updated ${file}`);
    }
});
