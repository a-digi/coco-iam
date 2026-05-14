import React from 'react';

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  className?: string;
}

const Pagination: React.FC<PaginationProps> = ({ currentPage, totalPages, onPageChange, className }) => {
  if (totalPages <= 1) return null;

  const getPageNumbers = () => {
    const pages: (number | string)[] = [];
    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) pages.push(i);
    } else {
      if (currentPage <= 4) {
        pages.push(1, 2, 3, 4, 5, '...', totalPages);
      } else if (currentPage >= totalPages - 3) {
        pages.push(1, '...', totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1, totalPages);
      } else {
        pages.push(1, '...', currentPage - 1, currentPage, currentPage + 1, '...', totalPages);
      }
    }
    return pages;
  };

  return (
    <nav className={`flex justify-center mt-6 ${className || ''}`} aria-label="Pagination">
      <ul className="inline-flex items-center space-x-1">
        <li>
          <button
            className="px-3 py-1 rounded-l bg-white border border-gray-300 text-gray-500 hover:bg-gray-100 disabled:opacity-50"
            onClick={() => onPageChange(currentPage - 1)}
            disabled={currentPage === 1}
            aria-label="Previous"
          >
            &laquo;
          </button>
        </li>
        {getPageNumbers().map((page, idx) => (
          <li key={idx}>
            {typeof page === 'number' ? (
              <button
                className={`px-3 py-1 border border-gray-300 ${
                  page === currentPage
                    ? 'bg-blue-600 text-white font-bold shadow'
                    : 'bg-white text-gray-700 hover:bg-blue-50'
                } rounded transition-colors duration-150`}
                onClick={() => onPageChange(page)}
                aria-current={page === currentPage ? 'page' : undefined}
              >
                {page}
              </button>
            ) : (
              <span className="px-3 py-1 text-gray-400">{page}</span>
            )}
          </li>
        ))}
        <li>
          <button
            className="px-3 py-1 rounded-r bg-white border border-gray-300 text-gray-500 hover:bg-gray-100 disabled:opacity-50"
            onClick={() => onPageChange(currentPage + 1)}
            disabled={currentPage === totalPages}
            aria-label="Next"
          >
            &raquo;
          </button>
        </li>
      </ul>
    </nav>
  );
};

export default Pagination;
